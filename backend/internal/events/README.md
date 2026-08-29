# Real-time event layer

The event bus is the internal boundary between monitoring/alert workers and real-time consumers. It intentionally drops events for a full subscriber buffer instead of blocking polling workers. Persistent delivery and cross-process fan-out should use NATS JetStream; this bus is for low-latency in-process delivery and WebSocket adapters.

Every event carries organization scope. HTTP/WebSocket handlers must authorize the organization before forwarding events to a client.
