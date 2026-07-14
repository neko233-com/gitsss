# gitsss Dashboard Design Baseline

Concept reference: `C:\Users\14170\.codex\generated_images\019f5efc-135b-7d62-b6d7-963bb1e380d6\exec-db30b8f3-9831-4a74-a892-da92662b76f7.png`

- Canvas: true white; graphite text; a single electric-blue focus state; black primary actions; neutral 1px rules and soft, low-elevation overlays.
- Typography: Geist variable; 32px page title; 18px section headings; 14px body; 12px timestamps and metadata.
- Layout: fixed 216px sidebar, open main canvas, and 312px event rail on desktop. The rail moves below the main content on narrow screens.
- Components: shadcn Radix Button, Avatar, Separator, Dialog and Command. gitsss supplies a bespoke rounded 1.65px icon family rather than product icons from an external set.
- First viewport copy: `今天`, `搜索或执行命令`, `待处理`, `构建失败`, `待发布`, `下一步`, `近期事件`, `清除预览`.
- Interaction: `Ctrl/Cmd+K` opens the command dialog; selecting a command produces an observable status message; clearing the event rail clears only the bounded LRU preview.
