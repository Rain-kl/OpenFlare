"use client"

import {useCallback, useEffect, useState} from "react"
import {useRouter} from "next/navigation"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {Kbd, KbdGroup} from "@/components/ui/kbd"
import {type SearchItem, searchItems} from "@/lib/utils/search-data"
import {FileText, Home, Settings, Shield} from "lucide-react"
import {useUser} from "@/contexts/user-context"

interface SearchDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const categoryIcons = {
  page: Home,
  feature: FileText,
  setting: Settings,
  admin: Shield,
}

const categoryLabels = {
  page: '页面',
  feature: '功能',
  setting: '设置',
  admin: '管理',
}

const getTips = (metaKey: string) => [
  (
    <>
      <span className="text-muted-foreground/80 lowercase">Tips: 还可以使用</span>
      <Kbd className="mx-1">/</Kbd>
      <span className="text-muted-foreground/80 lowercase">来打开此界面</span>
    </>
  ),
  (
    <>
      <span className="text-muted-foreground/80 lowercase">Tips: 使用</span>
      <Kbd className="mx-1">↑</Kbd>
      <Kbd className="mx-1">↓</Kbd>
      <span className="text-muted-foreground/80 lowercase">来切换选中项</span>
    </>
  ),
  (
    <>
      <span className="text-muted-foreground/80 lowercase">Tips: 按住</span>
      <Kbd className="mx-1">{metaKey}</Kbd>
      <span className="text-muted-foreground/80 lowercase">+</span>
      <Kbd className="mx-1">↵</Kbd>
      <span className="text-muted-foreground/80 lowercase">在新标签页打开</span>
    </>
  ),
  (
    <>
      <span className="text-muted-foreground/80 lowercase">你知道吗：搜索功能还在持续升级中</span>
    </>
  )
]

export function SearchDialog({ open, onOpenChange }: SearchDialogProps) {
  const router = useRouter()
  const { user } = useUser()
  const [search, setSearch] = useState('')
  const [currentTip, setCurrentTip] = useState<React.ReactNode>(null)
  const [results, setResults] = useState<SearchItem[]>([])
  const [metaKey, setMetaKey] = useState("⌘")
  const [isCtrlPressed, setIsCtrlPressed] = useState(false)

  useEffect(() => {
    if (typeof navigator !== 'undefined' && !navigator.userAgent?.includes("Mac")) {
      setMetaKey("Ctrl")
    }
  }, [])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Control' || e.key === 'Meta') {
        setIsCtrlPressed(true)
      }
    }
    const handleKeyUp = (e: KeyboardEvent) => {
      if (e.key === 'Control' || e.key === 'Meta') {
        setIsCtrlPressed(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
    }
  }, [])

  useEffect(() => {
    if (open) {
      const tips = getTips(metaKey)
      const randomTip = tips[Math.floor(Math.random() * tips.length)]
      setCurrentTip(randomTip)
    }
  }, [open, metaKey])

  useEffect(() => {
    const items = searchItems(search, user?.is_admin)
    setResults(items)
  }, [search, user?.is_admin])

  const handleSelect = useCallback((item: SearchItem, openInNewTab = false) => {
    onOpenChange(false)
    if (openInNewTab) {
      window.open(item.url, '_blank')
    } else {
      router.push(item.url)
    }
    setSearch('')
  }, [onOpenChange, router])

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        onOpenChange(!open)
      }

      if (e.key === '/'&& !e.ctrlKey && !e.metaKey && !e.altKey) {
        const target = e.target as HTMLElement
        const isEditing = target.tagName === 'INPUT' ||
                         target.tagName === 'TEXTAREA' ||
                         target.tagName === 'SELECT' ||
                         target.isContentEditable ||
                         target.closest('[contenteditable="true"]')

        if (!isEditing) {
          e.preventDefault()
          onOpenChange(true)
        }
      }
    }

    document.addEventListener('keydown', down)
    return () => document.removeEventListener('keydown', down)
  }, [open, onOpenChange])

  // Group results by category
  const groupedResults = results.reduce((acc, item) => {
    if (!acc[item.category]) {
      acc[item.category] = []
    }
    acc[item.category].push(item)
    return acc
  }, {} as Record<string, SearchItem[]>)

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      shouldFilter={false}
      className="max-w-[calc(100%-1.5rem)] sm:max-w-[500px] md:max-w-[540px]"
    >
      <CommandInput
        placeholder="搜索页面和功能..."
        value={search}
        onValueChange={setSearch}
      />
      <CommandList>
        <CommandEmpty>没有找到相关内容，换个词试试？</CommandEmpty>
        {Object.entries(groupedResults).map(([category, items]) => {
          const Icon = categoryIcons[category as keyof typeof categoryIcons]
          return (
            <CommandGroup key={category} heading={categoryLabels[category as keyof typeof categoryLabels]}>
              {items.map((item) => (
                <CommandItem
                  key={item.id}
                  value={item.title}
                  onSelect={() => handleSelect(item, isCtrlPressed)}
                  className="flex items-center justify-between"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted/70">
                      <Icon className="size-4" />
                    </div>
                    <div className="flex min-w-0 flex-col">
                      <span>
                        {item.matchRange ? (
                          <>
                            {item.title.substring(0, item.matchRange[0])}
                            <span className="text-primary font-bold">
                              {item.title.substring(item.matchRange[0], item.matchRange[1] + 1)}
                            </span>
                            {item.title.substring(item.matchRange[1] + 1)}
                          </>
                        ) : (
                          item.title
                        )}
                      </span>
                      <span className="truncate text-[11px] leading-4 text-muted-foreground">{item.description}</span>
                    </div>
                  </div>
                  <span className="ml-3 shrink-0 rounded-md bg-muted px-1.5 py-0.5 text-[10px] leading-none font-medium text-muted-foreground">
                    {categoryLabels[item.category as keyof typeof categoryLabels]}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          )
        })}
      </CommandList>
      <div className="hidden border-t bg-muted/20 px-4 py-2.5 md:flex items-center gap-4 text-[10px] text-muted-foreground uppercase tracking-wider font-medium select-none">
        <div className="flex items-center gap-1">
          <KbdGroup>
            {isCtrlPressed && <Kbd>{metaKey}</Kbd>}
            <Kbd>↵</Kbd>
          </KbdGroup>
          <span>{isCtrlPressed ? '在新标签页打开' : '打开'}</span>
        </div>
        <div className="flex items-center gap-1">
          <Kbd>Esc</Kbd>
          <span>关闭搜索界面</span>
        </div>
        <div className="ml-auto flex items-center gap-1">
          {currentTip}
        </div>
      </div>
    </CommandDialog>
  )
}
