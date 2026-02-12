import React, { useState } from 'react';
import { Palette, Moon, Sun } from 'lucide-react';
import { useTheme, themes, ThemeType } from '../contexts/ThemeContext';

export const ThemeSwitcher: React.FC = () => {
  const { theme, setTheme, colors } = useTheme();
  const [isOpen, setIsOpen] = useState(false);

  const darkThemes: ThemeType[] = ['military', 'ocean', 'purple', 'orange', 'dark'];
  const lightThemes: ThemeType[] = ['light', 'light-blue', 'light-green', 'light-rose'];

  const ThemeButton = ({ t }: { t: ThemeType }) => (
    <button
      onClick={() => {
        setTheme(t);
        setIsOpen(false);
      }}
      className={`w-full flex items-center gap-2 px-2 py-1.5 rounded text-sm transition-colors ${
        theme === t
          ? 'bg-[var(--accent)]/20 text-[var(--accent-2)]'
          : `${colors.isDark ? 'text-slate-300 hover:bg-slate-700/50' : 'text-slate-600 hover:bg-slate-200/50'}`
      }`}
    >
      <span
        className="w-3 h-3 rounded-full border border-black/10 shadow-sm"
        style={{ backgroundColor: themes[t].accent }}
      />
      <span className="flex-1 text-left">{themes[t].name}</span>
      {theme === t && (
        <span className="w-1.5 h-1.5 rounded-full bg-[var(--accent-2)]" />
      )}
    </button>
  );

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className={`p-2 rounded-lg fin-panel border fin-divider transition-colors ${
          colors.isDark
            ? 'text-slate-300 hover:text-white'
            : 'text-slate-600 hover:text-slate-900'
        }`}
        title="切换主题"
      >
        <Palette className="h-5 w-5" />
      </button>

      {isOpen && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setIsOpen(false)} />
          <div className="absolute right-0 top-full mt-2 z-50 fin-panel-strong border fin-divider rounded-lg p-2 min-w-[160px] shadow-xl">
            {/* 深色主题 */}
            <div className={`flex items-center gap-1.5 px-2 py-1 mb-1 text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
              <Moon size={12} />
              <span>深色主题</span>
            </div>
            {darkThemes.map((t) => (
              <ThemeButton key={t} t={t} />
            ))}

            {/* 分隔线 */}
            <div className="my-2 border-t fin-divider" />

            {/* 浅色主题 */}
            <div className={`flex items-center gap-1.5 px-2 py-1 mb-1 text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
              <Sun size={12} />
              <span>浅色主题</span>
            </div>
            {lightThemes.map((t) => (
              <ThemeButton key={t} t={t} />
            ))}
          </div>
        </>
      )}
    </div>
  );
};
