import { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { ChevronDown, Check, X } from 'lucide-react'

export interface SelectOption {
  value: string
  label: string
  disabled?: boolean
}

interface SelectProps {
  options: SelectOption[]
  value?: string | string[]
  defaultValue?: string | string[]
  onChange?: (value: string | string[]) => void
  placeholder?: string
  disabled?: boolean
  searchable?: boolean
  multiple?: boolean
  className?: string
  id?: string
  error?: string
}

export function Select({
  options,
  value: controlledValue,
  defaultValue,
  onChange,
  placeholder = 'Select an option...',
  disabled = false,
  searchable = false,
  multiple = false,
  className,
  id,
  error,
}: SelectProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [uncontrolledValue, setUncontrolledValue] = useState<string | string[]>(
    defaultValue || (multiple ? [] : '')
  )

  const containerRef = useRef<HTMLDivElement>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)

  const isControlled = controlledValue !== undefined
  const value = isControlled ? controlledValue : uncontrolledValue

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false)
        setSearchQuery('')
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Focus search input when opening
  useEffect(() => {
    if (isOpen && searchable && searchInputRef.current) {
      searchInputRef.current.focus()
    }
  }, [isOpen, searchable])

  const filteredOptions = searchable && searchQuery
    ? options.filter(option =>
        option.label.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : options

  const handleSelect = (optionValue: string) => {
    if (disabled) return

    let newValue: string | string[]

    if (multiple) {
      const currentValues = Array.isArray(value) ? value : []
      newValue = currentValues.includes(optionValue)
        ? currentValues.filter(v => v !== optionValue)
        : [...currentValues, optionValue]
    } else {
      newValue = optionValue
      setIsOpen(false)
      setSearchQuery('')
    }

    if (!isControlled) {
      setUncontrolledValue(newValue)
    }

    onChange?.(newValue)
  }

  const handleRemove = (optionValue: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (multiple && Array.isArray(value)) {
      const newValue = value.filter(v => v !== optionValue)
      if (!isControlled) {
        setUncontrolledValue(newValue)
      }
      onChange?.(newValue)
    }
  }

  const getSelectedLabels = () => {
    if (multiple && Array.isArray(value)) {
      return value
        .map(v => options.find(opt => opt.value === v)?.label)
        .filter(Boolean)
    }
    return options.find(opt => opt.value === value)?.label
  }

  const selectedLabels = getSelectedLabels()
  const hasValue = multiple
    ? Array.isArray(value) && value.length > 0
    : value !== ''

  return (
    <div ref={containerRef} className={cn('relative', className)}>
      <button
        id={id}
        type="button"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className={cn(
          'flex w-full items-center justify-between rounded-md border border-border bg-background px-3 py-2 text-sm ring-offset-background transition-colors',
          'focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2',
          disabled && 'cursor-not-allowed opacity-50',
          error && 'border-red-500 focus:ring-red-500',
          'hover:bg-muted/50'
        )}
      >
        <div className="flex flex-1 flex-wrap gap-1 overflow-hidden">
          {!hasValue && (
            <span className="text-muted-foreground">{placeholder}</span>
          )}
          {multiple && Array.isArray(selectedLabels) ? (
            selectedLabels.map((label, index) => (
              <span
                key={index}
                className="inline-flex items-center gap-1 rounded bg-primary/10 px-2 py-0.5 text-xs text-primary"
              >
                {label}
                <X
                  className="h-3 w-3 cursor-pointer hover:text-primary/70"
                  onClick={(e) => handleRemove(value[index], e)}
                />
              </span>
            ))
          ) : (
            <span className="truncate text-foreground">{selectedLabels}</span>
          )}
        </div>
        <ChevronDown
          className={cn(
            'ml-2 h-4 w-4 shrink-0 text-muted-foreground transition-transform',
            isOpen && 'rotate-180'
          )}
        />
      </button>

      {isOpen && (
        <div className="absolute z-50 mt-1 w-full rounded-md border border-border bg-card shadow-lg">
          {searchable && (
            <div className="border-b border-border p-2">
              <input
                ref={searchInputRef}
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search..."
                className="w-full rounded-md border border-border bg-background px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          )}

          <div className="max-h-60 overflow-auto p-1">
            {filteredOptions.length === 0 ? (
              <div className="px-3 py-2 text-center text-sm text-muted-foreground">
                No options found
              </div>
            ) : (
              filteredOptions.map((option) => {
                const isSelected = multiple
                  ? Array.isArray(value) && value.includes(option.value)
                  : value === option.value

                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => !option.disabled && handleSelect(option.value)}
                    disabled={option.disabled}
                    className={cn(
                      'flex w-full items-center justify-between rounded-sm px-3 py-2 text-sm transition-colors',
                      'hover:bg-muted focus:bg-muted focus:outline-none',
                      isSelected && 'bg-primary/10 text-primary',
                      option.disabled && 'cursor-not-allowed opacity-50'
                    )}
                  >
                    <span>{option.label}</span>
                    {isSelected && <Check className="h-4 w-4" />}
                  </button>
                )
              })
            )}
          </div>
        </div>
      )}

      {error && (
        <p className="mt-1 text-xs text-red-500">{error}</p>
      )}
    </div>
  )
}
