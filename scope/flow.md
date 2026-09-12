# Scope Flow

Execution order for building the scope tree and turning it into chunk candidates.

## 1. Parse the file

The parser reads the file and produces a flat list of `[]ExtractedEntity`.

- No application functions are called yet — just raw output.

## 2. Build the scope tree

`BuildScopeTreeFromEntities(entities)` — called first.

1. Splits the entities into **imports**, **exports**, and **scope entities**.
2. Sorts scope entities by start position.
3. Loops over each entity:
   - Calls `FindParentNode(root, entity)` — helper, called inside the build loop.
     - Internally calls `RangeContains(...)` — helper, called inside `FindParentNode`.
     - Returns the best parent node or `nil`.
   - Calls `NewScopeNode(entity, parent)` — helper, wraps an entity into a node.
   - Attaches the node as a child, or as a new root.
4. Returns `ScopeTree{Root, Imports, Exports, AllEntities}`.

> ✅ End of step 2 — the tree is fully built and sitting in memory.

## 3. Flatten the scope tree

`FlattenScopeTree(tree)` — called second, on the tree from step 2.

1. Walks the tree top to bottom (DFS).
2. Returns a flat `[]ScopeNode`, ordered.

> ✅ Now we have an ordered list of "chunk candidates".

## 4. Build context for each node

Loop over the flat list, for each node:

- Calls `GetAncestorChain(node)` — called per node, in this loop.
  - Walks `node.Parent` upward.
  - Returns `[]ScopeNode` (context / breadcrumb).
- Uses the ancestor chain to build a context label, e.g. `"inside Sub inside Calc"`.
- Sends the node's code plus context label to the AI / embedding model.

## Standalone helper

`FindScopeAtOffset(tree, offset)` — called anytime, independent of the chain above.

- Used when the editor asks: "what scope is the cursor at position X in?"
- Only needs the tree from step 2 — does not depend on the flatten / ancestor steps.