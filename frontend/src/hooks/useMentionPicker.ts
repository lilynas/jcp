import { useState, useRef, useCallback } from 'react';
import { AgentConfig } from '../services/strategyService';

interface UseMentionPickerProps {
  allAgents: AgentConfig[];
  onMentionSelect?: (agent: AgentConfig) => void;
}

interface UseMentionPickerReturn {
  // 状态
  mentionedAgents: string[];
  showMentionPicker: boolean;
  mentionSearchText: string;
  mentionSelectedIndex: number;
  filteredAgents: AgentConfig[];
  mentionListRef: React.RefObject<HTMLDivElement>;

  // 操作
  handleInputChange: (value: string, cursorPos: number) => string;
  handleKeyDown: (e: React.KeyboardEvent<HTMLInputElement>) => boolean;
  handleSelectMention: (agent: AgentConfig, currentQuery: string) => string;
  toggleMention: (agentId: string) => void;
  clearMentions: () => void;
  closePicker: () => void;
}

export const useMentionPicker = ({
  allAgents,
  onMentionSelect,
}: UseMentionPickerProps): UseMentionPickerReturn => {
  const [mentionedAgents, setMentionedAgents] = useState<string[]>([]);
  const [showMentionPicker, setShowMentionPicker] = useState(false);
  const [mentionSearchText, setMentionSearchText] = useState('');
  const [mentionStartIndex, setMentionStartIndex] = useState(-1);
  const [mentionSelectedIndex, setMentionSelectedIndex] = useState(0);
  const mentionListRef = useRef<HTMLDivElement>(null);

  // 过滤后的成员列表
  const filteredAgents = allAgents.filter(
    a => !mentionSearchText || a.name.toLowerCase().includes(mentionSearchText.toLowerCase())
  );

  // 滚动到选中项
  const scrollToSelected = useCallback((index: number) => {
    if (!mentionListRef.current) return;
    const items = mentionListRef.current.querySelectorAll('button');
    if (items[index]) {
      items[index].scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
  }, []);

  // 处理输入变化
  const handleInputChange = useCallback((value: string, cursorPos: number): string => {
    const lastChar = value[cursorPos - 1];

    // 检测是否刚输入了 @
    if (lastChar === '@') {
      setShowMentionPicker(true);
      setMentionStartIndex(cursorPos - 1);
      setMentionSearchText('');
      setMentionSelectedIndex(0);
      return value;
    }

    // 如果正在 @ 选择模式，更新搜索文本
    if (mentionStartIndex >= 0 && cursorPos > mentionStartIndex) {
      const searchText = value.substring(mentionStartIndex + 1, cursorPos);
      if (searchText.includes(' ')) {
        setShowMentionPicker(false);
        setMentionStartIndex(-1);
      } else {
        setMentionSearchText(searchText);
        setMentionSelectedIndex(0);
      }
    } else if (mentionStartIndex >= 0 && cursorPos <= mentionStartIndex) {
      setShowMentionPicker(false);
      setMentionStartIndex(-1);
    }

    return value;
  }, [mentionStartIndex]);

  // 处理键盘事件，返回 true 表示已处理
  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>): boolean => {
    if (!showMentionPicker || filteredAgents.length === 0) {
      return false;
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const newIndex = mentionSelectedIndex < filteredAgents.length - 1
        ? mentionSelectedIndex + 1 : 0;
      setMentionSelectedIndex(newIndex);
      setTimeout(() => scrollToSelected(newIndex), 0);
      return true;
    }

    if (e.key === 'ArrowUp') {
      e.preventDefault();
      const newIndex = mentionSelectedIndex > 0
        ? mentionSelectedIndex - 1 : filteredAgents.length - 1;
      setMentionSelectedIndex(newIndex);
      setTimeout(() => scrollToSelected(newIndex), 0);
      return true;
    }

    if (e.key === 'Escape') {
      e.preventDefault();
      setShowMentionPicker(false);
      setMentionStartIndex(-1);
      return true;
    }

    return false;
  }, [showMentionPicker, filteredAgents.length, mentionSelectedIndex, scrollToSelected]);

  // 选择 @ 成员
  const handleSelectMention = useCallback((agent: AgentConfig, currentQuery: string): string => {
    if (mentionStartIndex < 0) return currentQuery;

    const before = currentQuery.substring(0, mentionStartIndex);
    const after = currentQuery.substring(mentionStartIndex + 1 + mentionSearchText.length);
    const newQuery = `${before}@${agent.name} ${after}`;

    if (!mentionedAgents.includes(agent.id)) {
      setMentionedAgents(prev => [...prev, agent.id]);
    }

    setShowMentionPicker(false);
    setMentionStartIndex(-1);
    setMentionSearchText('');

    onMentionSelect?.(agent);

    return newQuery;
  }, [mentionStartIndex, mentionSearchText, mentionedAgents, onMentionSelect]);

  // 切换 @ 成员
  const toggleMention = useCallback((agentId: string) => {
    setMentionedAgents(prev =>
      prev.includes(agentId)
        ? prev.filter(id => id !== agentId)
        : [...prev, agentId]
    );
  }, []);

  // 清空所有 @
  const clearMentions = useCallback(() => {
    setMentionedAgents([]);
  }, []);

  // 关闭选择器
  const closePicker = useCallback(() => {
    setShowMentionPicker(false);
    setMentionStartIndex(-1);
  }, []);

  return {
    mentionedAgents,
    showMentionPicker,
    mentionSearchText,
    mentionSelectedIndex,
    filteredAgents,
    mentionListRef,
    handleInputChange,
    handleKeyDown,
    handleSelectMention,
    toggleMention,
    clearMentions,
    closePicker,
  };
};
