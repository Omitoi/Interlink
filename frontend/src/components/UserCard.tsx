// src/components/UserCard.tsx
import React from "react";
import type { UserSummary } from "../types/domain";
import Avatar from "./Avatar";
import s from "./UserCard.module.css";

export type UserCardProps = {
  user: UserSummary;
  actions?: React.ReactNode;
  onClick?: () => void;
  scorePercentage?: number; // Optional compatibility score percentage
};

function OnlineDot({ online }: { online?: boolean | undefined }) {
  if (online === undefined) return null;
  return <span className={online ? s.dotOnline : s.dotOffline} aria-label={online ? "online" : "offline"} />;
}

const UserCard: React.FC<UserCardProps> = ({ user, actions, onClick, scorePercentage }) => {
  const handleClick = () => {
    if (onClick) {
      onClick();
    }
  };

  const handleActionClick = (e: React.MouseEvent) => {
    e.stopPropagation();
  };

  return (
    <article className={s.card}>
      <div className={s.content} onClick={handleClick} role={onClick ? "button" : undefined} tabIndex={onClick ? 0 : undefined}>
        <div className={s.avatar}>
          <Avatar
            userId={user.id}
            alt={user.display_name}
            size={48}
          />
        </div>
        
        <div className={s.info}>
          <div className={s.nameRow}>
            <h3 className={s.name}>{user.display_name}</h3>
            <OnlineDot online={user.is_online} />
          </div>
          <div className={s.meta}>
            <span>Tap to view profile</span>
          </div>
        </div>

        {scorePercentage !== undefined && (
          <div className={s.score}>
            <span className={s.scoreText}>{Math.round(scorePercentage)}%</span>
          </div>
        )}
      </div>

      {actions && (
        <div className={s.actions} onClick={handleActionClick}>
          {actions}
        </div>
      )}
    </article>
  );
};

export default UserCard;