import React, { useState } from 'react';
import { TrendingUp, MessageSquare, MoreHorizontal, X, BarChart3, Flame, Palette, ChevronUp, ChevronDown } from 'lucide-react';
import type { MobileTab } from '../hooks/useResponsive';
import { useTheme, themes, ThemeType } from '../contexts/ThemeContext';

interface MobileNavProps {
  activeTab: MobileTab;
  onTabChange: (tab: MobileTab) => void;
  onShowLongHuBang?: () => void;
  onShowHotTrend?: () => void;
}

interface NavItem {
  id: MobileTab;
  icon: React.ElementType;
  label: string;
}

const mainNavItems: NavItem[] = [
  { id: 'chart', icon: TrendingUp, label: '行情' },
  { id: 'chat', icon: MessageSquare, label: 'AI' },
];

export const MobileNav: React.FC<MobileNavProps> = ({ 
  activeTab, 
  onTabChange,
  onShowLongHuBang,
  onShowHotTrend
}) => {
  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const [isDockVisible, setIsDockVisible] = useState(true);
  const [showThemePicker, setShowThemePicker] = useState(false);
  const { theme, setTheme } = useTheme();

  const handleMoreClick = () => {
    setShowMoreMenu(true);
  };

  const handleMenuItemClick = (action: () => void) => {
    action();
    setShowMoreMenu(false);
  };

  const toggleDock = () => {
    setIsDockVisible(!isDockVisible);
  };

  const handleThemeSelect = (newTheme: ThemeType) => {
    setTheme(newTheme);
    setShowThemePicker(false);
    setShowMoreMenu(false);
  };

  return (
    <>
      {/* Dock栏 */}
      <nav 
        className={`
          fixed left-0 right-0 h-16 fin-panel border-t fin-divider z-50 md:hidden safe-area-bottom
          transition-transform duration-300 ease-out
          ${isDockVisible ? 'translate-y-0' : 'translate-y-full'}
          bottom-0
        `}
      >
        <div className="flex justify-around items-center h-full px-2">
          {mainNavItems.map((item) => {
            const isActive = activeTab === item.id;
            const Icon = item.icon;
            
            return (
              <button
                key={item.id}
                onClick={() => onTabChange(item.id)}
                className={`
                  flex flex-col items-center justify-center w-16 h-14 rounded-xl
                  transition-all duration-200 active:scale-95
                  ${isActive 
                    ? 'text-accent-2 bg-accent/15 shadow-inner' 
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
                  }
                `}
                aria-label={item.label}
                aria-current={isActive ? 'page' : undefined}
              >
                <Icon size={22} strokeWidth={isActive ? 2.5 : 2} />
                <span className="text-[11px] mt-0.5 font-medium">{item.label}</span>
              </button>
            );
          })}
          
          {/* 更多功能按钮 */}
          <button
            onClick={handleMoreClick}
            className={`
              flex flex-col items-center justify-center w-16 h-14 rounded-xl
              transition-all duration-200 active:scale-95
              ${showMoreMenu 
                ? 'text-accent-2 bg-accent/15 shadow-inner' 
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
              }
            `}
            aria-label="更多功能"
          >
            <MoreHorizontal size={22} strokeWidth={2} />
            <span className="text-[11px] mt-0.5 font-medium">更多</span>
          </button>
        </div>
      </nav>

      {/* 收起按钮 - 固定在屏幕右侧，不跟随dock移动 */}
      {isDockVisible && (
        <button
          onClick={toggleDock}
          className="fixed bottom-20 right-4 z-[60] w-10 h-10 rounded-full fin-panel border fin-divider shadow-lg flex items-center justify-center hover:bg-slate-800/80 transition-all duration-200 active:scale-95 md:hidden"
          aria-label="收起导航栏"
        >
          <ChevronDown className="w-5 h-5 text-slate-400" />
        </button>
      )}

      {/* 浮动展开按钮 - 当dock收起时显示，固定在屏幕底部中央 */}
      {!isDockVisible && (
        <button
          onClick={toggleDock}
          className="fixed bottom-6 left-1/2 -translate-x-1/2 z-[60] w-12 h-12 rounded-full fin-panel border fin-divider shadow-lg flex items-center justify-center transition-all duration-200 active:scale-95 hover:bg-slate-800 md:hidden"
          aria-label="展开导航栏"
        >
          <ChevronUp className="w-5 h-5 text-slate-400" />
        </button>
      )}

      {/* 更多功能菜单弹窗 */}
      {showMoreMenu && (
        <div 
          className="fixed inset-0 z-[60] flex items-end justify-center bg-black/50 backdrop-blur-sm"
          onClick={() => setShowMoreMenu(false)}
        >
          <div 
            className={`
              w-full max-w-sm mx-4 fin-panel border fin-divider rounded-2xl shadow-2xl overflow-hidden animate-in slide-in-from-bottom-4 duration-200
              ${isDockVisible ? 'mb-20' : 'mb-4'}
            `}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-4 border-b fin-divider flex items-center justify-between">
              <span className="text-lg font-bold text-white">更多功能</span>
              <button 
                onClick={() => setShowMoreMenu(false)}
                className="p-2 rounded-lg hover:bg-slate-700/50 text-slate-400 transition-colors"
              >
                <X size={20} />
              </button>
            </div>
            
            <div className="p-4 grid grid-cols-3 gap-4">
              {/* 龙虎榜 */}
              {onShowLongHuBang && (
                <button
                  onClick={() => handleMenuItemClick(onShowLongHuBang)}
                  className="flex flex-col items-center gap-2 p-3 rounded-xl hover:bg-slate-800/50 transition-colors"
                >
                  <div className="w-12 h-12 rounded-full bg-red-500/20 flex items-center justify-center">
                    <BarChart3 className="w-6 h-6 text-red-400" />
                  </div>
                  <span className="text-xs text-slate-300">龙虎榜</span>
                </button>
              )}

              {/* 全网热点 */}
              {onShowHotTrend && (
                <button
                  onClick={() => handleMenuItemClick(onShowHotTrend)}
                  className="flex flex-col items-center gap-2 p-3 rounded-xl hover:bg-slate-800/50 transition-colors"
                >
                  <div className="w-12 h-12 rounded-full bg-orange-500/20 flex items-center justify-center">
                    <Flame className="w-6 h-6 text-orange-400" />
                  </div>
                  <span className="text-xs text-slate-300">全网热点</span>
                </button>
              )}

              {/* 主题切换 */}
              <button
                onClick={() => setShowThemePicker(true)}
                className="flex flex-col items-center gap-2 p-3 rounded-xl hover:bg-slate-800/50 transition-colors"
              >
                <div className="w-12 h-12 rounded-full bg-accent/20 flex items-center justify-center">
                  <Palette className="w-6 h-6 text-accent-2" />
                </div>
                <span className="text-xs text-slate-300">主题</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 主题选择器弹窗 */}
      {showThemePicker && (
        <div
          className="fixed inset-0 z-[70] flex items-end justify-center bg-black/50 backdrop-blur-sm"
          onClick={() => setShowThemePicker(false)}
        >
          <div
            className={`
              w-full max-w-sm mx-4 fin-panel border fin-divider rounded-2xl shadow-2xl overflow-hidden animate-in slide-in-from-bottom-4 duration-200
              ${isDockVisible ? 'mb-20' : 'mb-4'}
            `}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-4 border-b fin-divider flex items-center justify-between">
              <span className="text-lg font-bold text-white">选择主题</span>
              <button
                onClick={() => setShowThemePicker(false)}
                className="p-2 rounded-lg hover:bg-slate-700/50 text-slate-400 transition-colors"
              >
                <X size={20} />
              </button>
            </div>

            <div className="p-4 grid grid-cols-1 gap-2">
              {(Object.keys(themes) as ThemeType[]).map((themeKey) => {
                const themeInfo = themes[themeKey];
                const isActive = theme === themeKey;
                return (
                  <button
                    key={themeKey}
                    onClick={() => handleThemeSelect(themeKey)}
                    className={`
                      flex items-center gap-3 p-3 rounded-xl transition-colors
                      ${isActive ? 'bg-accent/20 border border-accent/50' : 'hover:bg-slate-800/50'}
                    `}
                  >
                    <div
                      className="w-8 h-8 rounded-full border-2"
                      style={{
                        backgroundColor: themeInfo.bg0,
                        borderColor: themeInfo.accent
                      }}
                    />
                    <span className={`text-sm ${isActive ? 'text-accent-2 font-medium' : 'text-slate-300'}`}>
                      {themeInfo.name}
                    </span>
                    {isActive && (
                      <span className="ml-auto text-accent-2 text-xs">当前</span>
                    )}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default MobileNav;
