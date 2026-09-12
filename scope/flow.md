1. Parser reads file → produce flat list []ExtractedEntity
   (no calling our functions yet, just raw output)

2. BuildScopeTreeFromEntities(entities) ← CALLED FIRST
   |
   |-- splits into imports / exports / scopeEntities
   |-- sorts scopeEntities by start position
   |-- loops each entity:
   | |
   | |-- calls FindParentNode(root, entity) ← HELPER, called INSIDE build loop
   | | |-- internally calls RangeContains(...) ← HELPER, called INSIDE FindParentNode
   | | |-- returns best parent node or nil
   | |
   | |-- calls NewScopeNode(entity, parent) ← HELPER, wraps entity into node
   | |-- attach node as child OR new root
   |
   |-- returns ScopeTree{Root, Imports, Exports, AllEntities}

   ✅ END OF STEP 2 — tree fully built, sitting in memory now.

3. FlattenScopeTree(tree) ← CALLED SECOND, on tree from step 2
   |
   |-- walks tree top to bottom (DFS)
   |-- returns flat []ScopeNode, ordered

   ✅ Now got ordered list of "chunk candidates"

4. Loop over that flat list, for each node:
   |
   |-- GetAncestorChain(node) ← CALLED per node, in this loop
   | |-- walks node.Parent upward
   | |-- returns []ScopeNode (context/breadcrumb)
   |
   |-- use ancestor chain to build context label ("inside Sub inside Calc")
   |-- send node's code + context label → AI/embedding model

(Separately, standalone, not part of above chain:)

FindScopeAtOffset(tree, offset) ← CALLED anytime, independent
used when editor ask "what scope is cursor at position X in"
just needs tree from step 2, don't depend on flatten/ancestor steps
