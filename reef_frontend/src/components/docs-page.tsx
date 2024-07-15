import { DocLang } from '@/lib/reef_std_doc';

interface DocsPageProps {
  docs: DocLang;
}

const DocsPage: React.FC<DocsPageProps> = ({ docs }) => {
  return (
    <div className="w-full h-full">
      {mapDescription(docs.description)}
      {docs.sections.map((section) => (
        <section key={section.name}>
          <h3 className="text-xl font-bold mt-4 mb-2">{section.name}</h3>
          {section.description ? mapDescription(section.description) : null}
          {section.entries.map((entry) => (
            <div key={entry.signature}>
              <p className="font-mono font-bold mt-4 mb-1">{entry.signature}</p>
              <div className="ml-2">{mapDescription(entry.description)}</div>
            </div>
          ))}
        </section>
      ))}
    </div>
  );
};
export default DocsPage;

const mapDescription = (desc: string[]) => desc.map((p) => <p key={p}>{p}</p>);
