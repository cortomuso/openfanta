import path from 'node:path';
import { visit } from 'unist-util-visit';

// Docs link to each other as relative `.md` files so they work on GitHub.
// Starlight serves pages at directory URLs, so resolve them against the docs root.
export default function rewriteMdLinks({ docsRoot, base }) {
  return (tree, file) => {
    const dir = path.dirname(path.relative(docsRoot, file.path));
    visit(tree, 'link', (node) => {
      const m = /^(?![a-z]+:|\/|#)(.+?)\.md(#.*)?$/.exec(node.url);
      if (!m) return;
      const target = path.posix.normalize(path.posix.join(dir, m[1]));
      node.url = `${base}/${target}/${m[2] ?? ''}`;
    });
  };
}
