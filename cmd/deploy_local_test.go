package cmd

import "testing"

func TestRewritePoetryInstallLine_RewritesInstallerLine(t *testing.T) {
	input := "FROM python:3.9-slim-bookworm\nRUN curl -sSL https://install.python-poetry.org | python3 -\n"

	got, changed := rewritePoetryInstallLine(input, "2.2.1")
	if !changed {
		t.Fatalf("expected rewrite to report change")
	}
	want := "FROM python:3.9-slim-bookworm\nRUN curl -sSL https://install.python-poetry.org | python3 - --version 2.2.1\n"
	if got != want {
		t.Fatalf("unexpected rewrite result\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestRewritePoetryInstallLine_NoChangeWhenPinned(t *testing.T) {
	input := "RUN curl -sSL https://install.python-poetry.org | python3 - --version 2.2.1\n"

	got, changed := rewritePoetryInstallLine(input, "2.2.1")
	if changed {
		t.Fatalf("expected no change for already pinned content")
	}
	if got != input {
		t.Fatalf("content changed unexpectedly")
	}
}

func TestRewritePoetryInstallLine_NoChangeWhenLineMissing(t *testing.T) {
	input := "RUN echo hello\n"

	got, changed := rewritePoetryInstallLine(input, "2.2.1")
	if changed {
		t.Fatalf("expected no change when installer line is missing")
	}
	if got != input {
		t.Fatalf("content changed unexpectedly")
	}
}
