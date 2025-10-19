import React from 'react';
import Toast from './Toast';
import { ToastInstance } from '../hooks/useToast';

interface ToastContainerProps {
  toasts: ToastInstance[];
}

const ToastContainer: React.FC<ToastContainerProps> = ({ toasts }) => {
  return (
    <>
      {toasts.map((toast) => (
        <Toast key={toast.id} {...toast} />
      ))}
    </>
  );
};

export default ToastContainer;