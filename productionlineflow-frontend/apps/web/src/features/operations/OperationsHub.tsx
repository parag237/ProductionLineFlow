type OperationsHubProps = {
  page: 'menu' | 'analysis';
  onOpenAnalysis: () => void;
  onOpenWorkLog: () => void;
};

export default function OperationsHub({ page, onOpenAnalysis, onOpenWorkLog }: OperationsHubProps) {
  return (
    <main className="workspace-shell">
      <section className="workspace-content operations-hub">
        <div className="workspace-heading">
          <div>
            <span className="panel-kicker">Operations</span>
            <h1>{page === 'analysis' ? 'Analysis' : 'Operations'}</h1>
          </div>
        </div>
        {page === 'menu' ? <div className="operations-choice-grid">
          <button className="operations-choice" onClick={onOpenAnalysis}>
            <span className="operations-choice-index">01</span>
            <span className="operations-choice-name">Analysis</span>
            <span className="operations-choice-arrow" aria-hidden="true">↗</span>
          </button>
          <button className="operations-choice" onClick={onOpenWorkLog}>
            <span className="operations-choice-index">02</span>
            <span className="operations-choice-name">Work log</span>
            <span className="operations-choice-arrow" aria-hidden="true">↗</span>
          </button>
        </div> : <section className="operations-analysis-empty">
          <span className="panel-kicker">Analysis</span>
          <h2>No analysis is available yet.</h2>
        </section>}
      </section>
    </main>
  );
}