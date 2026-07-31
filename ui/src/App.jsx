import React, { useState, useCallback } from 'react';
import WorkflowBuilder from './components/WorkflowBuilder';

export function useToast() {
  const [messages, setMessages] = useState([]);

  const toast = useCallback((text, type = 'info') => {
    const id = Date.now();
    setMessages((prev) => {
      const dup = prev.some((m) => m.text === text && id - m.id < 2000);
      if (dup) return prev;
      let next = type === 'error' ? prev.filter((m) => m.type !== 'error') : prev;
      if (next.length > 3) next = next.slice(-2);
      return [...next, { id, text, type }];
    });
    setTimeout(() => {
      setMessages((m) => m.filter((x) => x.id !== id));
    }, 4500);
  }, []);

  return { messages, toast };
}

function ToastStack({ messages }) {
  if (!messages.length) return null;
  return (
    <div className="toast-stack">
      {messages.map((m) => (
        <div key={m.id} className={`toast toast-${m.type}`}>{m.text}</div>
      ))}
    </div>
  );
}

export default function App() {
  const { messages, toast } = useToast();
  return (
    <>
      <WorkflowBuilder toast={toast} />
      <ToastStack messages={messages} />
    </>
  );
}
