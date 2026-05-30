# PASM -- Progressive Annotated Semantic Map

The idea came from the experience of being thrown into multiple projects you had no idea existed before. As engineers, we don't read everything at once, but we also don't read only what's immediately needed -- we seek broader context before deciding what deserves attention. If we treat an LLM as an engineer, it should have the same trait: be curious, explore the surroundings, then decide where to focus.

PASM is the foundation of lemongrass, and it has two layers:

1. **Deterministic semantic map** -- built by each language's parser in lemongrass. It creates an unexplored map of `path:symbol:kind:lines` entries, each called a node. A model can fetch this to determine the battleground.
2. **Annotated semantic map** -- built by any model that stumbles upon an unexplored node. It extracts a description of what the node does, its callers, and its dependencies. The description is then vectorized, enabling future models to search by context instead of path.

As models work continuously through the codebase, the semantic map gets annotated progressively.