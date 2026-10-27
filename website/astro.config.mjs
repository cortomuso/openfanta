import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightOpenAPI, { openAPISidebarGroups } from 'starlight-openapi';
import starlightLinksValidator from 'starlight-links-validator';
import { unified } from '@astrojs/markdown-remark';
import rewriteMdLinks from './plugins/rewrite-md-links.mjs';

const docsRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), 'src/content/docs');

const base = '/openfanta';

export default defineConfig({
  site: 'https://cortomuso.github.io',
  base,
  markdown: { processor: unified({ remarkPlugins: [[rewriteMdLinks, { docsRoot, base }]] }) },
  integrations: [
    starlight({
      title: 'OpenFanta',
      description: 'An open fantacalcio data framework.',
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/cortomuso/openfanta' }],
      editLink: { baseUrl: 'https://github.com/cortomuso/openfanta/edit/main/docs/' },
      plugins: [
        starlightLinksValidator({ exclude: ['/openfanta/api/'] }),
        starlightOpenAPI([{ base: 'api', label: 'API reference', schema: '../api/openapi.yaml' }]),
      ],
      sidebar: [
        { label: 'Start here', items: ['getting-started', 'project-layout'] },
        { label: 'Guides', items: ['guides/changing-ruleset-values', 'guides/adding-a-provider'] },
        { label: 'Reference', items: ['rulesets', 'api-overview'] },
        ...openAPISidebarGroups,
        { label: 'Design', items: ['design'] },
        {
          label: 'League settings research',
          collapsed: true,
          items: [{ autogenerate: { directory: 'research/league-settings' } }],
        },
      ],
    }),
  ],
});
