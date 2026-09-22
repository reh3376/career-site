import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { AccessNote } from "@/components/access-note";
import { getArticle, listArticles } from "@/lib/articles";

// Statically prerender each known slug at build time. Combined with
// force-static, the reader page becomes an edge-cacheable HTML page
// per article, no per-request markdown parse. Public since 2026-09-22.
export const dynamic = "force-static";

export async function generateStaticParams(): Promise<{ slug: string }[]> {
  const articles = await listArticles();
  return articles.map((a) => ({ slug: a.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const a = await getArticle(slug);
  if (!a) return { title: "Article not found" };
  return {
    title: a.title,
    description: a.subtitle,
  };
}

export default async function ArticleReaderPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const article = await getArticle(slug);
  if (!article) notFound();

  return (
    <article className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        <Link
          href="/articles"
          className="text-signal no-underline hover:text-accent"
        >
          ← writing
        </Link>
      </p>

      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {article.title}
      </h1>
      {article.subtitle ? (
        <p className="mt-4 text-lg leading-snug text-ink-2">
          {article.subtitle}
        </p>
      ) : null}
      {article.date ? (
        <p className="mt-4 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          {article.date}
        </p>
      ) : null}

      <div className="prose-article mt-10 text-ink-2">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            h1: ({ children }) => (
              <h2
                className="font-display mt-12 text-2xl leading-snug text-ink sm:text-3xl"
                style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
              >
                {children}
              </h2>
            ),
            h2: ({ children }) => (
              <h3
                className="font-display mt-10 text-xl leading-snug text-ink"
                style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
              >
                {children}
              </h3>
            ),
            h3: ({ children }) => (
              <h4 className="mt-8 font-semibold text-ink">{children}</h4>
            ),
            p: ({ children }) => (
              <p className="mt-5 text-base leading-relaxed">{children}</p>
            ),
            ul: ({ children }) => (
              <ul className="mt-5 list-disc space-y-2 pl-6">{children}</ul>
            ),
            ol: ({ children }) => (
              <ol className="mt-5 list-decimal space-y-2 pl-6">{children}</ol>
            ),
            blockquote: ({ children }) => (
              <blockquote className="mt-6 border-l-2 border-accent bg-accent/5 py-2 pl-4 italic text-ink">
                {children}
              </blockquote>
            ),
            a: ({ children, href }) => (
              <a
                href={href}
                className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
                rel="noopener noreferrer"
                target={href?.startsWith("http") ? "_blank" : undefined}
              >
                {children}
              </a>
            ),
            code: ({ children, className }) => {
              const isBlock = /language-/.test(className ?? "");
              if (isBlock) {
                return (
                  <code className="block overflow-x-auto rounded border border-line-strong bg-paper-2 px-3 py-2 font-mono text-[13px] text-ink">
                    {children}
                  </code>
                );
              }
              return (
                <code className="rounded bg-paper-2 px-1 py-0.5 font-mono text-[0.9em] text-ink">
                  {children}
                </code>
              );
            },
            hr: () => <hr className="my-10 border-t border-line" />,
            table: ({ children }) => (
              <div className="mt-6 overflow-x-auto">
                <table className="w-full min-w-full border-collapse text-sm">
                  {children}
                </table>
              </div>
            ),
            th: ({ children }) => (
              <th className="border-b border-line-strong bg-paper-2 px-3 py-2 text-left font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                {children}
              </th>
            ),
            td: ({ children }) => (
              <td className="border-b border-line px-3 py-2 align-top">
                {children}
              </td>
            ),
          }}
        >
          {article.body}
        </ReactMarkdown>
      </div>

      <p className="mt-16 border-t border-line pt-6 text-sm text-ink-3">
        <Link
          href="/articles"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          More writing →
        </Link>
      </p>

      <AccessNote />
    </article>
  );
}
