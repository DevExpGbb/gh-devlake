# GitHub Foreman — Workflow Reference

This document contains interaction diagrams and sequential user flows for the GitHub Foreman agent team.

## Agent Interaction Model

This diagram shows which agents run where (local VS Code vs. cloud GitHub) and how they interact with each other, PRs, and the human developer.

```mermaid
graph TB
    subgraph "Human Developer"
        H["👤 You<br/>(review + merge only)"]
    end

    subgraph "Local — VS Code Agents"
        F["🏗️ GitHub Foreman<br/>(coordinator)"]
        WR["🔍 Wave Reviewer<br/>(subagent, read-only)"]
        DW["📝 Docs Writer<br/>(subagent)"]
        GD["⚙️ go-developer<br/>(existing)"]
        PR_AGENT["✨ prettify<br/>(existing)"]
    end

    subgraph "Cloud — GitHub Platform"
        CA1["☁️ Coding Agent<br/>Session A"]
        CA2["☁️ Coding Agent<br/>Session B"]
        CRA["📋 Code Review Agent"]
        CI["🔧 CI<br/>(go build/vet/test)"]
    end

    subgraph "GitHub Repository"
        IS["📂 Issues &<br/>Milestones"]
        PR1["PR #1<br/>copilot/issue-X"]
        PR2["PR #2<br/>copilot/issue-Y"]
        INST[".github/copilot-instructions.md"]
    end

    %% Human interacts only with Foreman and PRs
    H -->|"prompt: plan / dispatch / review / merge"| F
    H -->|"approve"| PR1
    H -->|"approve"| PR2

    %% Foreman reads issues
    F -->|"github/list_issues,<br/>github/issue_read"| IS

    %% Foreman dispatches to cloud coding agents
    F -->|"github/assign_copilot_to_issue<br/>(base_branch, model)"| CA1
    F -->|"github/assign_copilot_to_issue"| CA2

    %% Cloud agents create PRs
    CA1 -->|"creates + pushes"| PR1
    CA2 -->|"creates + pushes"| PR2

    %% Cloud agents read repo instructions
    CA1 -.->|"reads"| INST
    CA2 -.->|"reads"| INST

    %% Code review is primarily triggered by PR ready-for-review ruleset
    F -->|"mark ready for review"| PR1
    F -->|"mark ready for review"| PR2
    PR1 -->|"ruleset assigns Code Review Agent"| CRA
    PR2 -->|"ruleset assigns Code Review Agent"| CRA
    F -->|"github/request_copilot_review<br/>(fallback trigger)"| CRA
    CRA -->|"review comments"| PR1
    CRA -->|"review comments"| PR2
    CRA -.->|"reads review guidance"| INST

    %% CI runs automatically on every push
    PR1 -->|"triggers"| CI
    PR2 -->|"triggers"| CI
    CI -->|"check results"| PR1
    CI -->|"check results"| PR2

    %% Foreman runs local subagents
    F -->|"runSubagent"| WR
    F -->|"runSubagent"| DW

    %% Wave Reviewer reads PRs
    WR -->|"github/pull_request_read"| PR1
    WR -->|"github/pull_request_read"| PR2
    WR -->|"consistency report"| F

    %% Docs Writer updates docs
    DW -->|"edits README,<br/>AGENTS.md"| DW
    DW -->|"doc report"| F

    %% Foreman summarizes for human
    F -->|"wave summary:<br/>ready for review"| H
```

## Sequential User Flow

This diagram shows the step-by-step flow a human developer experiences when working with the GitHub Foreman.

```mermaid
sequenceDiagram
    actor Human
    participant Foreman as 🏗️ GitHub Foreman
    participant Issues as 📂 GitHub Issues
    participant CodingAgent as ☁️ Coding Agent (Cloud)
    participant PRs as 📄 Pull Requests
    participant CodeReview as 📋 Code Review Agent
    participant CI as 🔧 CI (GitHub Actions)
    participant WaveReview as 🔍 Wave Reviewer
    participant Docs as 📝 Docs Writer

    Note over Human,Foreman: ═══ PHASE 1: PLAN ═══
    Human->>Foreman: "Plan the next wave"
    Foreman->>Issues: Read open issues + dependencies
    Issues-->>Foreman: Issue details, labels, milestones
    Foreman->>Foreman: Build dependency graph
    Foreman->>Foreman: Select models per issue
    Foreman-->>Human: Wave plan proposal<br/>(issues, models, branches)
    Human->>Foreman: "Approved — dispatch"

    Note over Human,CodingAgent: ═══ PHASE 2: DISPATCH ═══
    par Parallel dispatch
        Foreman->>CodingAgent: Assign issue #A<br/>(best reasoning model, base: main)
        Foreman->>CodingAgent: Assign issue #B<br/>(fast Codex model, base: main)
    end
    Foreman-->>Human: "Dispatched 2 issues.<br/>Agents working in background."

    Note over Foreman,PRs: ═══ PHASE 2b: MONITOR (automatic) ═══
    Foreman->>Foreman: Sleep 5 minutes (initial wait)
    loop Poll every 2min until all PRs created
        Foreman->>CodingAgent: get_copilot_job_status
        CodingAgent-->>Foreman: ⏳ Working / 📄 PR created / ❌ Failed
        alt Still working
            Foreman->>Foreman: Sleep 2 minutes
        end
    end

    CodingAgent->>PRs: PR #A created (draft)
    CodingAgent->>PRs: PR #B created (draft)
    Foreman-->>Human: "All PRs are in. Starting reviews."

    Note over Foreman,PRs: ═══ PHASE 3: CODE REVIEW LOOP (automatic) ═══
    loop Until no actionable comments remain
        Foreman->>PRs: Mark PRs ready for review
        Foreman->>CodeReview: Request review (fallback if ruleset doesn't trigger)
        CodeReview-->>PRs: Review comments
        Foreman->>Foreman: Sleep 5 min, poll until reviews complete
        Foreman->>PRs: Collect + judge review comments
        alt Actionable comments found
            Foreman->>PRs: Post @copilot <fix description>
            Foreman->>Foreman: Sleep 3 min, poll for new commits
        else No actionable comments
            Note over Foreman: Exit review loop → Phase 4
        end
    end

    Note over Foreman,CI: ═══ PHASE 4: CI GATE (automatic) ═══
    CI->>PRs: go build / go vet / go test (Linux/Windows/macOS)
    loop Poll every 2min until checks complete
        Foreman->>PRs: github/pull_request_read (method: "get_checks")
        alt All green
            Note over Foreman: Proceed to Phase 5
        else Any red
            Foreman->>PRs: Post @copilot <failure summary + fix request>
            Foreman->>Foreman: Sleep 3 min, poll for new commits
            Note over Foreman: Loop back to Phase 3
        end
    end

    Note over Foreman,Docs: ═══ PHASE 5: DOCS & CONSISTENCY ═══
    par Post-review checks
        Foreman->>WaveReview: Check cross-PR consistency
        Foreman->>Docs: Verify docs are updated
    end
    WaveReview-->>Foreman: Consistency findings
    Docs-->>Foreman: Doc completeness report

    Note over Human,Foreman: ═══ PHASE 6: HUMAN GATE ═══
    Foreman-->>Human: Wave summary:<br/>✅ Code review: clean<br/>✅ CI: passing<br/>✅ Cross-PR: consistent<br/>✅ Docs: updated

    Human->>Foreman: "LGTM — merge them"

    Note over Human,PRs: ═══ PHASE 7: ADVANCE ═══
    Foreman->>PRs: gh pr merge --squash --delete-branch (PR #A)
    PRs-->>Foreman: ✅ Merged, branch deleted
    Foreman->>PRs: gh pr merge --squash --delete-branch (PR #B)
    PRs-->>Foreman: ✅ Merged, branch deleted
    Foreman->>Foreman: Update state, find next wave
    Foreman-->>Human: "Next wave: issues #C, #D..."
```

## Model Selection Quick Reference

| Issue Type | Recommended Model | Rationale |
|-----------|-------------------|-----------|
| Complex refactor (multi-file, architectural) | Best Claude / reasoning model | Best reasoning for large codebases |
| Docs, help text, straightforward additions | Fastest Codex model | Fast, good for well-scoped work |
| Test generation, coverage improvements | Balanced Codex model | Balanced capability and speed |
| Multiple parallel dispatches | Auto | Avoids rate limiting |
| Unknown / general | **Auto** (default) | Always safe, adapts to available models |

> **Note:** Model availability changes over time. The table above describes *categories* of models, not specific versions. When in doubt, use `Auto` and let the platform choose. The human can always override with a specific model name.
