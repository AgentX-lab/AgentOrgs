"""AgentOrgs Matrix overlay for hermes-worker.

Keeps hermes-agent's native mautrix transport and adds:
  * outbound ``m.mentions`` enrichment
  * DM / group allow-lists
  * history buffering in group rooms
  * vision gating when the model has no image input
"""
