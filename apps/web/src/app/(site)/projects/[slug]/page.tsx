import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { getProject, listProjects, STATUS_LABEL } from "@/lib/projects";

// A project's detail page (FR-CNT-05): problem, approach, outcomes,
// links, and the writing related to it.
//
// The body is markdown with its own headings rather than fixed fields,
// because the row names four things a project page should answer and
// forcing every project into exactly those four would produce empty
// sections for the ones that have nothing to say under a heading. An
// empty "Architecture" is worse than no architecture section.

export async function generateStaticParams() {
  const projects = await listProjects();
  return projects.map((p) => ({ slug: p.slug }));
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const project = await getProject(slug);
  if (!project) return { title: "Project not found" };
  return {
    title: project.title,
    description: project.summary,
    alternates: { canonical: `/projects/${project.slug}` },
    openGraph: {
      type: "article",
      title: project.title,
      description: project.summary,
      url: `/projects/${project.slug}`,
    },
  };
}

export default async function ProjectPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const project = await getProject(slug);
  if (!project) notFound();

  return (
    <div className="mx-auto max-w-2xl px-6 py-24 sm:px-10 sm:py-32">
      <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
        <Link href="/projects" className="text-ink-3 no-underline hover:text-accent">
          projects
        </Link>
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {project.title}
      </h1>
      <p className="mt-5 text-lg leading-relaxed text-ink-2">
        {project.summary}
      </p>

      <dl className="mt-8 grid gap-x-6 gap-y-3 border-y border-line py-5 font-mono text-[11px] tracking-[0.1em] uppercase sm:grid-cols-[8rem_1fr]">
        <dt className="text-ink-3">My role</dt>
        <dd className="m-0 text-ink normal-case">{project.role}</dd>
        <dt className="text-ink-3">Status</dt>
        <dd className="m-0 text-ink normal-case">
          {STATUS_LABEL[project.status]}
        </dd>
        {project.started ? (
          <>
            <dt className="text-ink-3">Since</dt>
            <dd className="m-0 text-ink tabular-nums">
              {project.started}
              {project.ended ? ` to ${project.ended}` : null}
            </dd>
          </>
        ) : null}
        {project.repo ? (
          <>
            <dt className="text-ink-3">Source</dt>
            <dd className="m-0">
              <a
                href={project.repo}
                target="_blank"
                rel="noopener noreferrer"
                className="text-accent normal-case underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
              >
                {project.repo.replace(/^https:\/\//, "")}
              </a>
            </dd>
          </>
        ) : null}
      </dl>

      {project.technologies.length ? (
        <ul className="mt-6 flex flex-wrap gap-2">
          {project.technologies.map((t) => (
            <li
              key={t}
              className="border border-line bg-paper-2 px-2.5 py-1 font-mono text-[10px] tracking-[0.1em] text-ink-3 uppercase"
            >
              {t}
            </li>
          ))}
        </ul>
      ) : null}

      <div className="mt-10 text-ink-2">
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            h2: ({ children }) => (
              <h2
                className="font-display mt-10 text-xl leading-snug text-ink"
                style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
              >
                {children}
              </h2>
            ),
            p: ({ children }) => (
              <p className="mt-5 text-base leading-relaxed">{children}</p>
            ),
            ul: ({ children }) => (
              <ul className="mt-5 list-disc space-y-2 pl-5">{children}</ul>
            ),
            code: ({ children }) => (
              <code className="bg-paper-2 px-1 py-0.5 font-mono text-[0.9em] text-ink">
                {children}
              </code>
            ),
            a: ({ href, children }) => (
              <a
                href={href}
                target={href?.startsWith("http") ? "_blank" : undefined}
                rel="noopener noreferrer"
                className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
              >
                {children}
              </a>
            ),
          }}
        >
          {project.body}
        </ReactMarkdown>
      </div>

      <p className="mt-14 border-t border-line pt-6 font-mono text-[11px] tracking-[0.14em] uppercase">
        <Link href="/projects" className="text-accent no-underline hover:underline">
          All projects
        </Link>
      </p>
    </div>
  );
}
