---
name: add-inbox-handler
description: Use when the task consumes a message, an event or a webhook from another service in this Avero application. It states the handler shape, the dedupe key and the test that proves one effect for two deliveries.
---

# Add an inbox handler

An inbox handler reads one message and writes one change. The dedupe row and
the change commit together, so a message that arrives two times gives one
effect.

The inbox consumer of the host arrives with S8. The module contract already
holds the interface, so the shape of the work is known.

## Steps

1. Write the handler in the slice that owns the message:

   ```go
   func (m *Module) OnCaptured(ctx context.Context, e events.PaymentCaptured) error
   ```

2. Implement `InboxHandlers() []module.InboxHandler` on the module. Each
   handler states its name and the type of the message that it reads. The name
   and the message identifier form the dedupe key.

3. Write the change inside the transaction that the consumer opens. Never
   acknowledge before the commit.

4. Write the test. It must prove four things:
   - one message that arrives two times gives one change;
   - one message that reaches two handlers gives two changes;
   - a handler that returns an error rolls back and acknowledges nothing;
   - a message that fails too often reaches the dead letter.

## Rules

- A handler is idempotent. It reads the state before it writes.
- A handler holds no timer and no retry of its own. The consumer owns both.

## The command that proves the work

```
avero verify
```
