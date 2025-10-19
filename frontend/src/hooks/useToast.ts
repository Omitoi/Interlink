import { useState, useCallback } from 'react';
import { ToastProps } from '../components/Toast';

export interface ToastInstance extends ToastProps {
  id: string;
}

export function useToast() {
  const [toasts, setToasts] = useState<ToastInstance[]>([]);

  const removeToast = useCallback((id: string) => {
    setToasts(prev => prev.filter(toast => toast.id !== id));
  }, []);

  const addToast = useCallback((toast: Omit<ToastProps, 'onClose'>) => {
    const id = Math.random().toString(36).substring(2, 9);
    const newToast: ToastInstance = {
      ...toast,
      id,
      onClose: () => removeToast(id),
    };
    
    setToasts(prev => [...prev, newToast]);
    return id;
  }, [removeToast]);

  const confirm = useCallback((
    title: string,
    message?: string,
    options?: {
      confirmText?: string;
      cancelText?: string;
      type?: 'danger' | 'warning' | 'info';
    }
  ): Promise<boolean> => {
    return new Promise((resolve) => {
      addToast({
        type: options?.type || 'info',
        title,
        ...(message && { message }),
        confirmText: options?.confirmText || 'Yes',
        cancelText: options?.cancelText || 'Cancel',
        onConfirm: () => resolve(true),
        onCancel: () => resolve(false),
      });
    });
  }, [addToast]);

  const success = useCallback((title: string, message?: string) => {
    return addToast({
      type: 'success',
      title,
      ...(message && { message }),
      autoClose: true,
      duration: 4000,
    });
  }, [addToast]);

  const error = useCallback((title: string, message?: string) => {
    return addToast({
      type: 'danger',
      title,
      ...(message && { message }),
    });
  }, [addToast]);

  const info = useCallback((title: string, message?: string) => {
    return addToast({
      type: 'info',
      title,
      ...(message && { message }),
      autoClose: true,
      duration: 4000,
    });
  }, [addToast]);

  return {
    toasts,
    addToast,
    removeToast,
    confirm,
    success,
    error,
    info,
  };
}