---
description: "Use when writing or modifying terminal output in CLI commands — covers line spacing, headers, emoji, indentation, and end-to-end UX flow."
applyTo: "cmd/**/*.go"
---
# Terminal Output Standards

The terminal is the UI. Every `fmt.Print` call is a UX decision.

## Line Spacing Rules (Mandatory)

1. **Blank line before every step.** Every emoji-prefixed step starts with `\n`:
   ```go
   fmt.Println("\n🔍 Discovering DevLake instance...")
   ```

2. **Blank line before AND after separators.** `───`, `═══`, and phase banners need breathing room:
   ```go
   fmt.Println("\n" + strings.Repeat("─", 50))
   fmt.Println("✅ Connection configured!")
   fmt.Println(strings.Repeat("─", 50))
   fmt.Println()
   ```

3. **Blank line after completion banners:**
   ```go
   fmt.Println("════════════════════════════════════════")
   fmt.Println("  ✅ Setup Complete!")
   fmt.Println("════════════════════════════════════════")
   fmt.Println()
   ```

4. **Sub-items stay tight under their parent.** 3-space indent, no blank lines between them:
   ```go
   fmt.Println("\n📡 Creating GitHub connection...")
   fmt.Printf("   Endpoint: %s\n", endpoint)
   fmt.Printf("   Token:    %s\n", masked)
   fmt.Printf("   ✅ Created (ID=%d)\n", conn.ID)
   ```

5. **Phase banners get blank lines on both sides:**
   ```go
   fmt.Println("\n╔══════════════════════════════════════╗")
   fmt.Println("║  PHASE 1: Configure Connections      ║")
   fmt.Println("╚══════════════════════════════════════╝")
   fmt.Println()
   ```

6. **Blank line before interactive prompts.** When a prompt (`Select`, `ReadLine`, `Confirm`, input line) follows output, add a blank line so the prompt doesn't jam against the previous text:
   ```go
   fmt.Println("   These connections were found.")
   fmt.Println()
   selected := prompt.SelectMultiWithDefaults(...)
   ```
   Inside the prompt package itself, add `\n` before "Enter" input lines that follow a list of options.

## Header Standards — Unicode `═` at 40 Characters

**Top-level banner:**
```go
fmt.Println("\n════════════════════════════════════════")
fmt.Println("  DevLake — Command Title")
fmt.Println("════════════════════════════════════════")
```

**Completion banner:**
```go
fmt.Println("\n════════════════════════════════════════")
fmt.Println("  ✅ Action Complete!")
fmt.Println("════════════════════════════════════════")
fmt.Println()
```

Never use ASCII `=` — always Unicode `═`. Width is always 40.

## Indentation

- **Steps**: column 0, emoji prefix
- **Sub-items**: 3-space indent (`"   "`)
- **Banner content**: 2-space indent (`"  "`)
- **Bullets**: 2-space + `•` (`"  • item"`)
- **Numbered**: 2-space + number (`"  1. step"`)

## End-to-End UX

Before adding any output, mentally walk the full terminal scroll. The pattern is:
**banner → steps with progress → summary**

The user should always know what just happened and what comes next.
