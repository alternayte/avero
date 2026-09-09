# Add an inbox handler

The inbox consumer arrives with S8 of the host. The module contract already
holds the interface, so the shape of the work is known.

## Steps

1. Write the handler type in the slice that owns the message.
2. Implement `InboxHandlers() []module.InboxHandler` on the module. Each
   handler states its name and the type of the message that it reads.
3. Write the test that delivers one message two times and asserts one
   mutation.

## The command that proves the work

```
avero verify
```
