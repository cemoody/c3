const imageExts = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp']);
const markdownExts = new Set(['md', 'markdown', 'mdx']);
const textExts = new Set([
  'txt', 'log', 'csv', 'json', 'yaml', 'yml', 'toml', 'xml',
  'py', 'js', 'ts', 'jsx', 'tsx', 'sh', 'bash', 'zsh',
  'go', 'rs', 'c', 'cpp', 'h', 'hpp', 'java', 'rb', 'php',
  'css', 'scss', 'less', 'html', 'sql', 'r', 'lua',
  'env', 'ini', 'conf', 'cfg', 'properties',
  'svelte', 'vue', 'swift', 'kt', 'scala',
  'makefile', 'dockerfile',
]);
const openInTabExts = new Set(['pdf']);
const textFileNames = new Set(['makefile', 'dockerfile', 'rakefile', 'gemfile', 'procfile']);

function getExt(name: string): string {
  return name.includes('.') ? name.split('.').pop()!.toLowerCase() : '';
}

export function isImage(name: string): boolean {
  return imageExts.has(getExt(name));
}

export function isMarkdown(name: string): boolean {
  return markdownExts.has(getExt(name));
}

export function isTextFile(name: string): boolean {
  return textExts.has(getExt(name)) || textFileNames.has(name.toLowerCase());
}

export function isPreviewable(name: string): boolean {
  const ext = getExt(name);
  if (ext === 'html' || ext === 'pdf') return false;
  return isImage(name) || isMarkdown(name) || isTextFile(name);
}

export function isPlot(name: string): boolean {
  return /\.(png|jpg|jpeg|gif|webp|svg|pdf|html|md)$/i.test(name);
}

export type FileType = 'image' | 'markdown' | 'text' | 'open-in-tab' | 'download';

export function getFileType(name: string): FileType {
  const ext = getExt(name);
  if (imageExts.has(ext)) return 'image';
  if (markdownExts.has(ext)) return 'markdown';
  if (textExts.has(ext) || textFileNames.has(name.toLowerCase())) return 'text';
  if (openInTabExts.has(ext)) return 'open-in-tab';
  return 'download';
}
