import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'Food Nutrition AI',
  description: 'AI-powered food nutrition assistant using RAG + local LLM',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="h-full">
      <body className="h-full bg-slate-900 antialiased">{children}</body>
    </html>
  );
}
