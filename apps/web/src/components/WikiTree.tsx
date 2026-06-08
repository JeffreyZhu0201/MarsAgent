interface WikiDoc { slug: string; title: string; category: string; source: string; updated_at: string }

export function WikiTree({
  docs,
  selected,
  onSelect,
}: {
  docs: WikiDoc[]
  selected: WikiDoc | null
  onSelect: (d: WikiDoc) => void
}) {
  const grouped = docs.reduce<Record<string, WikiDoc[]>>((acc, d) => {
    (acc[d.category] ||= []).push(d); return acc
  }, {})

  return (
    <div className="mt-4 space-y-4 text-sm">
      {Object.entries(grouped).map(([cat, catDocs]) => (
        <div key={cat}>
          <div className="font-medium text-slate-500 uppercase text-xs mb-1">{cat}</div>
          {catDocs.map(d => (
            <div
              key={d.slug}
              className={`cursor-pointer px-2 py-1 rounded hover:bg-slate-100 ${
                selected?.slug === d.slug ? 'bg-slate-100 font-medium' : ''
              }`}
              onClick={() => onSelect(d)}
            >
              {d.title.slice(0, 40)}
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
