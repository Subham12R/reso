export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-6xl flex-col px-6 py-8 sm:px-10">
      <header className="flex items-center justify-between border-b border-[var(--line)] pb-5">
        <span className="font-serif text-3xl">reso</span>
        <span className="text-sm text-[var(--muted)]">Private watch rooms</span>
      </header>
      <section className="flex flex-1 flex-col justify-center py-20">
        <p className="mb-5 text-sm text-[var(--muted)]">
          Temporary. Private. No accounts.
        </p>
        <h1 className="max-w-3xl font-serif text-5xl leading-none sm:text-7xl">
          Watch together without making a place to stay.
        </h1>
        <p className="mt-8 max-w-xl text-lg leading-8 text-[var(--muted)]">
          Reso is a focused room for sharing a screen, talking, and leaving no
          permanent room behind.
        </p>
      </section>
      <footer className="border-t border-[var(--line)] pt-5 text-sm text-[var(--muted)]">
        Foundation build — room creation is coming next.
      </footer>
    </main>
  );
}
