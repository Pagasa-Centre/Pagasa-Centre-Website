---
name: demo-claude-greeter
description: Demo — greet new PRs (runs on Anthropic / Claude)
on:
  GitHubWebhook:
    match: "event.github_webhook.event_type == 'pull_request' && (event.github_webhook.payload.action == 'ready_for_review' || (event.github_webhook.payload.action == 'opened' && event.github_webhook.payload.pull_request.draft == false))"
model: claude-3-5-haiku-latest
---
A new non-draft pull request was just opened. The `## Context` block below has the PR details.

Post ONE short, friendly greeting comment on the PR using the `pull_request_review_write` tool. Begin the comment with "🧠 Claude demo check:" and in one sentence say the PR was received. Pass `commitId` set to the PR head SHA from `event.github_webhook.payload.pull_request.head.sha`. Do nothing else.
