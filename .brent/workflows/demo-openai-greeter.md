---
name: demo-openai-greeter
description: Demo — greet new PRs (runs on OpenAI)
on:
  GitHubWebhook:
    match: "event.github_webhook.event_type == 'pull_request' && (event.github_webhook.payload.action == 'ready_for_review' || (event.github_webhook.payload.action == 'opened' && event.github_webhook.payload.pull_request.draft == false))"
model: gpt-5.4-mini
---
A new non-draft pull request was just opened. The `## Context` block below has the PR details.

Post ONE short, friendly greeting comment on the PR using the `pull_request_review_write` tool. Begin the comment with "🤖 OpenAI demo check:" and in one sentence say the PR was received and will be reviewed. Pass `commitId` set to the PR head SHA from `event.github_webhook.payload.pull_request.head.sha` so the comment is marked outdated if new commits are pushed. Do nothing else.
