# E2E ONLY. Loaded into kind as agentorgs/mock-llm:local (never pulled in-cluster).
FROM python:3.11-slim
COPY server.py /app/server.py
ENV PYTHONUNBUFFERED=1
EXPOSE 6556
CMD ["python3", "-u", "/app/server.py"]
