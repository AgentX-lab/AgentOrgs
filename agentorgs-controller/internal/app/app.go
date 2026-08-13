package app

import (
	"context"
	"fmt"
	"time"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/collaboration"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/controller"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/server"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	matrixprovider "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/collaboration/matrix"
	k8sbackend "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/execution/kubernetes"
	openclawadapter "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/runtime/openclaw"
	miniostore "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/storage/minio"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// App wires controller-runtime, providers, engine, and HTTP server.
type App struct {
	cfg      config.Config
	mgr      ctrl.Manager
	registry *provider.Registry
	engine   *collaboration.Engine
	matrix   *matrixprovider.Provider
	http     *server.HTTPServer
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	ctrl.SetLogger(zap.New())
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := agentorgsv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: cfg.MetricsBindAddr},
	})
	if err != nil {
		return nil, err
	}

	registry := provider.NewRegistry()
	k8sExec := k8sbackend.NewBackend(mgr.GetClient(), cfg)
	storage, err := miniostore.NewStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("init storage provider: %w", err)
	}
	openclaw := openclawadapter.NewAdapter(cfg, storage, mgr.GetClient())
	matrix := matrixprovider.NewProvider(cfg, mgr.GetClient())

	registry.RegisterExecution(k8sExec)
	registry.RegisterRuntime(openclaw)
	registry.RegisterCollaboration(matrix)
	registry.RegisterStorage(storage)

	reader := &collaboration.K8sReader{Client: mgr.GetClient()}
	engine := &collaboration.Engine{Registry: registry, Client: reader}

	if err := controller.SetupMemberReconciler(mgr, registry); err != nil {
		return nil, err
	}
	if err := controller.SetupGroupReconciler(mgr); err != nil {
		return nil, err
	}
	if err := controller.SetupCollaborationReconciler(mgr); err != nil {
		return nil, err
	}
	if err := controller.SetupPolicyReconciler(mgr); err != nil {
		return nil, err
	}

	appService := matrixprovider.NewAppServiceHandler(cfg.MatrixAppServiceHSToken, cfg.Namespace, mgr.GetClient(), matrix)
	httpServer := &server.HTTPServer{
		Engine:     engine,
		AppService: appService,
		Addr:       cfg.HTTPAddr,
	}

	app := &App{
		cfg:      cfg,
		mgr:      mgr,
		registry: registry,
		engine:   engine,
		matrix:   matrix,
		http:     httpServer,
	}

	go func() {
		_ = matrix.Subscribe(ctx, func(ctx context.Context, event protocol.CollaborationEvent) error {
			if event.RunID != "" {
				_, err := engine.ReceiveMessage(ctx, event)
				return err
			}
			if len(event.Targets) == 0 {
				return fmt.Errorf("inbound event requires targets")
			}
			_, err := engine.StartCollaboration(ctx, event.Namespace, event.Collaboration, event.Source.Member, event.Targets, event.Payload)
			return err
		})
	}()

	go func() {
		_ = httpServer.Start()
	}()

	go func() {
		if !mgr.GetCache().WaitForCacheSync(ctx) {
			return
		}
		setup := matrixprovider.NewSetup(cfg, mgr.GetClient(), matrix.API)
		// CRs may be applied after the controller starts; keep retrying setup.
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			if err := setup.EnsureReady(ctx); err != nil {
				ctrl.Log.Error(err, "matrix setup failed")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return app, nil
}

func (a *App) Start(ctx context.Context) error {
	return a.mgr.Start(ctx)
}
