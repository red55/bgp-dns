let's implement dns cache clearing command in `bgp-dnsctl`.
the command should be called from shell as `bgp-dnsctl cache clear`.
you have to implement corresponding functions in `internal/bgp` package. take a look on style how the `bgp-dnsctl cache list` was implemented.

give me detailed plan on your implementation. each phase of plan should be a single git commit. commit and proceed to the next phase only after my confirmation.
write unit test for code you implemented. run unit test in separate agent and separate git branch. if unit tests succeeded merge it into working branch.

Use `git-expert` agent for git operations.

Each phase of plan has to be called in separate agent to avoid main context window pollution.

use QWEN.md as context memory for this project.

if you have corresponding questions feel free to ask me.