export default function ComingSoon({ title, desc }: { title: string; desc: string }) {
  return (
    <div className="grid gap-4">
      <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
      <div className="card p-5 text-center">
        <p className="font-semibold">Segera hadir</p>
        <p className="hint">{desc}</p>
      </div>
    </div>
  );
}
