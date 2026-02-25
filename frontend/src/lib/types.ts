export type Pane = { index: string; currentCommand: string; target: string; claudeState?: string; currentPath?: string };
export type Window = { index: string; name: string; panes: Pane[] };
export type Session = { name: string; windows: Window[] };
