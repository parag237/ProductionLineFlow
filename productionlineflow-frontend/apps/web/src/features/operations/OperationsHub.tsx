import type { Permissions } from '../../types';
import OperationsAnalysis from './OperationsAnalysis';

type OperationsHubProps = {
  page: 'menu' | 'analysis';
  permissions: Permissions;
  onOpenAnalysis: () => void;
  onOpenWorkLog: () => void;
};

export default function OperationsHub({ page, permissions, onOpenAnalysis, onOpenWorkLog }: OperationsHubProps) {
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
        </div> : <OperationsAnalysis permissions={permissions} />}
      </section>
    </main>
  );
}