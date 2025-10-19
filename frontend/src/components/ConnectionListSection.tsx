import React from "react";
import s from "./ConnectionListSection.module.css";

export type ConnectionListSectionProps = {
  title: string;
  loading: boolean;
  emptyText: string;
  children: React.ReactNode;
  className?: string; // (not in use)
};

const ConnectionListSection: React.FC<ConnectionListSectionProps> = ({
  title,
  loading,
  emptyText,
  children,
  className,
}) => {
  const sectionClass = className ? `${s.section} ${className}` : s.section;

  return (
    <section className={sectionClass}>
      <header className={s.header}>
        <h2 className={s.title}>{title}</h2>
        {loading && (
          <span
            className={s.dotSpinner}
            role="status"
            aria-live="polite"
            aria-label="Loading"
            title="Loading"
          >
            <i /><i /><i />
          </span>
        )}
      </header>

      {loading ? (
        <div className={s.loading} aria-busy="true" aria-live="polite">
           Loading…
        </div>
      ) : React.Children.count(children) === 0 ? (
        <div className={s.empty} aria-live="polite">
          {emptyText}
        </div>
      ) : (
        <div className={s.list} role="list">
          {children}
        </div>
      )}
    </section>
  );
};

export default ConnectionListSection;
