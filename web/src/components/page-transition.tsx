import "./page-transition.css";

interface PageTransitionProps {
  readonly animationKey: string;
  readonly children: React.ReactNode;
}

export function PageTransition({
  animationKey,
  children,
}: PageTransitionProps): JSX.Element {
  return (
    <div className="page-transition-wrapper">
      <div key={animationKey} className="page-transition-content">
        {children}
      </div>
    </div>
  );
}
