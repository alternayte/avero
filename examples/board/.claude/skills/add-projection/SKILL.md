---
name: add-projection
description: Use when the task adds a read model, a summary table, a counter or a report that follows a change of another table in this Avero application. It states where the read model is written and how the test proves it.
---

# Add a projection

A projection reads a change and writes a read model. The projection runner of
the host arrives with S9. Until then, the read model is written in the same
transaction as the change that it follows, so the two never drift.

## Steps

1. Write the table of the read model:

   ```
   avero migrate new <name>_read_model
   ```

2. Write the update of the read model in the store of the feature that owns
   the change. Both writes run inside the transaction of the request, so they
   commit together.

3. Write the read of the read model as its own method, so a handler reads one
   row and joins nothing.

4. Write the test. It must prove three things:
   - the read model matches the source after a change;
   - a failed request leaves the read model as it was;
   - a second identical change gives the same read model.

5. Apply the migration with `avero migrate up`.

## Rules

- Never write the read model outside the transaction of the change.
- A projection holds no business rule. It holds a shape that a page reads.

## The command that proves the work

```
avero verify
```
