import { CSSTransition, TransitionGroup } from "react-transition-group";
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
    <TransitionGroup className="page-transition-wrapper">
      <CSSTransition
        key={animationKey}
        timeout={{
          enter: 400,
          exit: 300,
        }}
        classNames="page"
      >
        <div className="page-transition-content">{children}</div>
      </CSSTransition>
    </TransitionGroup>
  );
}
