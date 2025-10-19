import React, { useEffect, useState } from 'react';
import styles from './Toast.module.css';

export interface ToastProps {
  type?: 'info' | 'success' | 'warning' | 'danger';
  title: string;
  message?: string;
  onConfirm?: () => void;
  onCancel?: () => void;
  confirmText?: string;
  cancelText?: string;
  autoClose?: boolean;
  duration?: number;
  onClose?: () => void;
}

const Toast: React.FC<ToastProps> = ({
  type = 'info',
  title,
  message,
  onConfirm,
  onCancel,
  confirmText = 'Yes',
  cancelText = 'Cancel',
  autoClose = false,
  duration = 5000,
  onClose,
}) => {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // Animate in with a small delay to allow the component to mount
    const animationTimer = setTimeout(() => setIsVisible(true), 10);

    if (autoClose && duration > 0 && onClose) {
      const timer = setTimeout(() => {
        setIsVisible(false);
        setTimeout(() => {
          onClose();
        }, 200);
      }, duration);
      return () => {
        clearTimeout(timer);
        clearTimeout(animationTimer);
      };
    }
    return () => clearTimeout(animationTimer);
  }, [autoClose, duration, onClose]);

  const handleClose = () => {
    setIsVisible(false);
    setTimeout(() => {
      onClose?.();
    }, 200); // Wait for animation
  };

  const handleConfirm = () => {
    onConfirm?.();
    handleClose();
  };

  const handleCancel = () => {
    onCancel?.();
    handleClose();
  };

  const isConfirmation = onConfirm || onCancel;

  return (
    <div className={`${styles.overlay} ${isVisible ? styles.visible : ''}`} onClick={handleClose}>
      <div 
        className={`${styles.toast} ${styles[type]} u-card`} 
        onClick={(e) => e.stopPropagation()}
        role="dialog" 
        aria-modal="true"
        aria-labelledby="toast-title"
        aria-describedby={message ? "toast-message" : undefined}
      >
        <div className={styles.content}>
          <h3 id="toast-title" className={styles.title}>
            {title}
          </h3>
          {message && (
            <p id="toast-message" className={styles.message}>
              {message}
            </p>
          )}
        </div>

        {isConfirmation ? (
          <div className={styles.actions}>
            {onCancel && (
              <button
                type="button"
                className="u-btn"
                onClick={handleCancel}
              >
                {cancelText}
              </button>
            )}
            {onConfirm && (
              <button
                type="button"
                className={`u-btn ${type === 'danger' ? 'u-btn--danger' : 'u-btn--primary'}`}
                onClick={handleConfirm}
              >
                {confirmText}
              </button>
            )}
          </div>
        ) : (
          <div className={styles.actions}>
            <button
              type="button"
              className="u-btn"
              onClick={handleClose}
            >
              Close
            </button>
          </div>
        )}
      </div>
    </div>
  );
};

export default Toast;