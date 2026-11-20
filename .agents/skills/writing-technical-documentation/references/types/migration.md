# Migration and upgrade guide

Open with who must act and by when. List breaking changes with before and after code, then ordered steps, a rollback path, a way to verify, and common errors. State the supported source and target versions exactly, and point to a codemod or script if one exists.

Take breaking changes from the diff and the changelog, and run the steps on a copy of a real setup. Readers are changing a working system, so an untested migration step does real damage.

````markdown
# Migrate from <vA> to <vB>

Who must act: <who>. Deadline: <date or version>.

## Breaking changes

### <Change>

Before:

```<lang>
<code>
```

After:

```<lang>
<code>
```

## Steps

1. <Action.>

## Roll back

<Steps.>

## Verify

<Check.>

## Common errors

| Error | Fix |
|---|---|
| `<error>` | <fix> |
````
