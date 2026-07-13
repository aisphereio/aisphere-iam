from pathlib import Path

path = Path("internal/data/memory.go")
text = path.read_text()

old = 'func (r *MemoryControlPlaneRepository) CreateGrant(ctx context.Context, grant *GrantModel, audit *GrantAuditModel, event *OutboxEventModel) error {'
new = 'func (r *MemoryControlPlaneRepository) CreateGrant(ctx context.Context, grant *GrantModel, audit *GrantAuditModel, events ...*OutboxEventModel) error {'
if old not in text:
    raise SystemExit("missing MemoryControlPlaneRepository.CreateGrant")
text = text.replace(old, new, 1)
start = text.index(new)
tail = text[start:]
old_save = '\tr.saveEvent(event)\n\treturn nil\n}'
new_save = '\tfor _, event := range events {\n\t\tr.saveEvent(event)\n\t}\n\treturn nil\n}'
if old_save not in tail:
    raise SystemExit("missing CreateGrant event save")
tail = tail.replace(old_save, new_save, 1)
text = text[:start] + tail

old = 'func (r *MemoryControlPlaneRepository) RevokeGrant(ctx context.Context, id string, revokedAt time.Time, audit *GrantAuditModel, event *OutboxEventModel) error {'
new = 'func (r *MemoryControlPlaneRepository) RevokeGrant(ctx context.Context, id string, revokedAt time.Time, audit *GrantAuditModel, events ...*OutboxEventModel) error {'
if old not in text:
    raise SystemExit("missing MemoryControlPlaneRepository.RevokeGrant")
text = text.replace(old, new, 1)
start = text.index(new)
tail = text[start:]
if old_save not in tail:
    raise SystemExit("missing RevokeGrant event save")
tail = tail.replace(old_save, new_save, 1)
text = text[:start] + tail
path.write_text(text)

lifecycle = Path("internal/data/project_lifecycle.go")
content = lifecycle.read_text()
content += '''

func (r *MemoryControlPlaneRepository) CreateOutboxEvents(_ context.Context, events ...*OutboxEventModel) error {
\tr.mu.Lock()
\tdefer r.mu.Unlock()
\tfor _, event := range events {
\t\tr.saveEvent(event)
\t}
\treturn nil
}

func (r *MemoryControlPlaneRepository) ListOutboxEvents(_ context.Context, opts ListOptions) ([]OutboxEventModel, error) {
\tr.mu.RLock()
\tdefer r.mu.RUnlock()
\tout := make([]OutboxEventModel, 0, len(r.events))
\tfor _, event := range r.events {
\t\tif statusOK(event.Status, opts.Status) {
\t\t\tout = append(out, *clone(event))
\t\t}
\t}
\treturn out, nil
}
'''
lifecycle.write_text(content)
