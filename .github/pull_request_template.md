<!--
  PR body = reviewer surface (lives in GitHub UI, ephemeral).
  Per-commit messages = the durable archeology; squash auto-concats them
  into the merge commit body via repo setting `COMMIT_MESSAGES`.

  Keep this body slim. Put what + why + how-it-was-built in commit messages.
  Put what reviewers need *while the PR is open* here.
-->

## Summary

<!-- 1-3 sentences. What changed and why. Link to spec / handover if relevant. -->

## Test plan

- [ ] `go build ./...` clean
- [ ] `go test ./...` pass
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` empty (no unformatted files)
- [ ] Hardware: ran on a real Kobo, no regressions in the golden path (delete if not device-facing)
- [ ] Manual smoke: <feature-specific scenario>

## Reviewer notes

<!-- Anything that helps a reviewer scan faster. Trade-offs considered,
     alternatives rejected, follow-ups already filed. Delete if nothing non-obvious. -->
