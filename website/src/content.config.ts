import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { docsSchema } from '@astrojs/starlight/schema';

// The Markdown lives in ../docs so GitHub and the site read the same files (src/content/docs is a symlink).
export const collections = {
  docs: defineCollection({
    loader: glob({ pattern: '**/*.{md,mdx}', base: './src/content/docs' }),
    schema: docsSchema(),
  }),
};
