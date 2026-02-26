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
        QA["🧪 QA Enforcer<br/>(subagent, terminal)"]
        GD["⚙️ go-developer<br/>(existing)"]
        PR_AGENT["✨ prettify<br/>(existing)"]
    end

    subgraph "Cloud — GitHub Platform"
        CA1["☁️ Coding Agent<br/>Session A"]
        CA2["☁️ Coding Agent<br/>Session B"]
        CRA["📋 Code Review Agent"]
    end

    subgraph "GitHub Repository"
        IS["📂 Issues &<br/>Milestones"]
        PR1["PR #1<br/>copilot/issue-X"]
        PR2["PR #2<br/>copilot/issue-Y"]
        CI["copilot-instructions.md"]
    end

    %% Human interacts only with Foreman and PRs
    H -->|"prompt: plan / dispatch / review / merge"| F
    H -->|"approve"| PR1
    H -->|"approve"| PR2

    %% Foreman reads issues
    F -->|"MCP: list_issues,<br/>issue_read"| IS

    %% Foreman dispatches to cloud coding agents
    F -->|"MCP: assign_copilot_to_issue<br/>(base_branch, model)"| CA1
    F -->|"MCP: assign_copilot_to_issue"| CA2

    %% Cloud agents create PRs
    CA1 -->|"creates + pushes"| PR1
    CA2 -->|"creates + pushes"| PR2

    %% Cloud agents read repo instructions
    CA1 -.->|"reads"| CI
    CA2 -.->|"reads"| CI

    %% Foreman triggers code review
    F -->|"MCP: request_copilot_review"| CRA
    CRA -->|"review comments"| PR1
    CRA -->|"review comments"| PR2
    CRA -.->|"reads review guidance"| CI

    %% Foreman runs local subagents
    F -->|"runSubagent"| WR
    F -->|"runSubagent"| DW
    F -->|"runSubagent"| QA

    %% Wave Reviewer reads PRs
    WR -->|"MCP: pull_request_read"| PR1
    WR -->|"MCP: pull_request_read"| PR2
    WR -->|"consistency report"| F

    %% QA Enforcer runs tests locally
    QA -->|"go test / build / vet"| QA
    QA -->|"test results"| F

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
    participant WaveReview as 🔍 Wave Reviewer
    participant QA as 🧪 QA Enforcer
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

    Note over Human,Docs: ═══ PHASE 3: REVIEW (automatic) ═══

    par Automated reviews
        Foreman->>CodeReview: Request review on PR #A
        Foreman->>CodeReview: Request review on PR #B
    end
    CodeReview-->>PRs: Review comments

    par Local subagent checks
        Foreman->>WaveReview: Check cross-PR consistency
        Foreman->>QA: Run go build / test / vet + CLI smoke tests
        Foreman->>Docs: Verify docs are updated
    end

    WaveReview-->>Foreman: Consistency findings
    QA-->>Foreman: Build + test + CLI results
    Docs-->>Foreman: Doc completeness report

    Foreman->>PRs: Collect Code Review Agent comments
    PRs-->>Foreman: Review comments by severity

    Note over Human,Foreman: ═══ PHASE 4: HUMAN GATE ═══
    Foreman-->>Human: Wave summary:<br/>✅ Code review: clean<br/>✅ Cross-PR: consistent<br/>✅ Tests: passing<br/>⚠️ README: missing 1 entry

    Human->>Foreman: "Fix the README"
    Foreman->>PRs: Post @copilot fix comment

    Note over Foreman,QA: ═══ PHASE 4b: FIX LOOP (automatic) ═══
    Foreman->>Foreman: Sleep 3 minutes
    loop Poll every 2min for new commits
        Foreman->>PRs: Check for new commits
        alt New commits found
            Foreman->>QA: Re-run QA Enforcer
            QA-->>Foreman: Updated results
            Foreman->>PRs: Re-collect review comments
        else Still waiting
            Foreman->>Foreman: Sleep 2 minutes
        end
    end
    Foreman-->>Human: Fix status:<br/>✅ Fix applied, QA passing

    Note over Human,PRs: ═══ PHASE 5: MERGE ═══
    Human->>Foreman: "LGTM — merge them"
    Foreman->>PRs: gh pr merge --squash --delete-branch (PR #A)
    PRs-->>Foreman: ✅ Merged, branch deleted
    Foreman->>PRs: gh pr merge --squash --delete-branch (PR #B)
    PRs-->>Foreman: ✅ Merged, branch deleted

    Note over Human,Foreman: ═══ PHASE 6: ADVANCE ═══
    Human->>Foreman: "Wave complete — next"
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
