import React, { useState, useCallback, createContext, useContext } from 'react';
import { Check, AlertCircle, Loader2 } from 'lucide-react';

type ToastType = 'success' | 'error' | 'loading';

interface ToastItem {
  id: string;
  type: ToastType;
  message: string;
}

interface ToastContextType {
  showToast: (type: ToastType, message: string, duration?: number) => string;
  hideToast: (id: string) => void;
}

const ToastContext = createContext<ToastContextType | null>(null);

export const useToast = () => {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast must be used within ToastProvider');
  }
  return context;
};

export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const hideToast = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  const showToast = useCallback((type: ToastType, message: string, duration = 2000) => {
    const id = `toast-${Date.now()}`;
    setToasts(prev => [...prev, { id, type, message }]);
    if (type !== 'loading' && duration > 0) {
      setTimeout(() => hideToast(id), duration);
    }
    return id;
  }, [hideToast]);

  return (
    <ToastContext.Provider value={{ showToast, hideToast }}>
      {children}
      <ToastContainer toasts={toasts} />
    </ToastContext.Provider>
  );
};

const ToastContainer: React.FC<{ toasts: ToastItem[] }> = ({ toasts }) => {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2">
      {toasts.map(toast => (
        <ToastItemView key={toast.id} toast={toast} />
      ))}
    </div>
  );
};

const ToastItemView: React.FC<{ toast: ToastItem }> = ({ toast }) => {
  const icons = {
    success: <Check className="h-4 w-4 text-green-400" />,
    error: <AlertCircle className="h-4 w-4 text-red-400" />,
    loading: <Loader2 className="h-4 w-4 text-blue-400 animate-spin" />,
  };

  const bgColors = {
    success: 'bg-green-500/10 border-green-500/30',
    error: 'bg-red-500/10 border-red-500/30',
    loading: 'bg-blue-500/10 border-blue-500/30',
  };

  return (
    <div className={`flex items-center gap-2 px-4 py-2 rounded-lg border ${bgColors[toast.type]} backdrop-blur-sm animate-slide-in`}>
      {icons[toast.type]}
      <span className="text-sm text-white">{toast.message}</span>
    </div>
  );
};
