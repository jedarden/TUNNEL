import { useState, useEffect } from 'react'
import { FormField, Label, HelpText } from '../ui/Form'
import { Switch } from '../ui/Switch'
import { useUIStore } from '@/stores/ui'
import { Sun, Moon, Monitor } from 'lucide-react'
import { cn } from '@/lib/utils'

const accentColors = [
  { name: 'Blue', value: '#3b82f6', class: 'bg-blue-500' },
  { name: 'Purple', value: '#a855f7', class: 'bg-purple-500' },
  { name: 'Green', value: '#22c55e', class: 'bg-green-500' },
  { name: 'Orange', value: '#f97316', class: 'bg-orange-500' },
  { name: 'Pink', value: '#ec4899', class: 'bg-pink-500' },
  { name: 'Teal', value: '#14b8a6', class: 'bg-teal-500' },
]

const fontSizes = [
  { label: 'Small', value: '14px' },
  { label: 'Medium', value: '16px' },
  { label: 'Large', value: '18px' },
]

export function AppearanceSettings() {
  const { theme, setTheme } = useUIStore()
  const [accentColor, setAccentColor] = useState('#3b82f6')
  const [fontSize, setFontSize] = useState('16px')
  const [compactMode, setCompactMode] = useState(false)
  const [showAnimations, setShowAnimations] = useState(true)

  useEffect(() => {
    // Load saved preferences from localStorage
    const savedAccent = localStorage.getItem('accent-color')
    const savedFontSize = localStorage.getItem('font-size')
    const savedCompact = localStorage.getItem('compact-mode')
    const savedAnimations = localStorage.getItem('show-animations')

    if (savedAccent) setAccentColor(savedAccent)
    if (savedFontSize) setFontSize(savedFontSize)
    if (savedCompact) setCompactMode(savedCompact === 'true')
    if (savedAnimations) setShowAnimations(savedAnimations === 'true')
  }, [])

  const handleThemeChange = (newTheme: 'light' | 'dark' | 'system') => {
    setTheme(newTheme)
  }

  const handleAccentColorChange = (color: string) => {
    setAccentColor(color)
    localStorage.setItem('accent-color', color)
    // In a real app, you'd apply this color to CSS custom properties
    document.documentElement.style.setProperty('--accent-color', color)
  }

  const handleFontSizeChange = (size: string) => {
    setFontSize(size)
    localStorage.setItem('font-size', size)
    document.documentElement.style.setProperty('--base-font-size', size)
  }

  const handleCompactModeChange = (checked: boolean) => {
    setCompactMode(checked)
    localStorage.setItem('compact-mode', checked.toString())
  }

  const handleShowAnimationsChange = (checked: boolean) => {
    setShowAnimations(checked)
    localStorage.setItem('show-animations', checked.toString())
    if (!checked) {
      document.documentElement.classList.add('reduce-motion')
    } else {
      document.documentElement.classList.remove('reduce-motion')
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">Appearance Settings</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Customize the look and feel of the application.
        </p>
      </div>

      <div className="space-y-6">
        {/* Theme Selection */}
        <FormField>
          <Label description="Choose your preferred color scheme">Theme</Label>
          <div className="grid grid-cols-3 gap-3">
            <button
              onClick={() => handleThemeChange('light')}
              className={cn(
                'flex flex-col items-center gap-2 rounded-lg border-2 p-4 transition-colors',
                theme === 'light'
                  ? 'border-primary bg-primary/5'
                  : 'border-border hover:bg-muted'
              )}
            >
              <Sun className="h-6 w-6" />
              <span className="text-sm font-medium">Light</span>
            </button>

            <button
              onClick={() => handleThemeChange('dark')}
              className={cn(
                'flex flex-col items-center gap-2 rounded-lg border-2 p-4 transition-colors',
                theme === 'dark'
                  ? 'border-primary bg-primary/5'
                  : 'border-border hover:bg-muted'
              )}
            >
              <Moon className="h-6 w-6" />
              <span className="text-sm font-medium">Dark</span>
            </button>

            <button
              onClick={() => handleThemeChange('system')}
              className={cn(
                'flex flex-col items-center gap-2 rounded-lg border-2 p-4 transition-colors',
                theme === 'system'
                  ? 'border-primary bg-primary/5'
                  : 'border-border hover:bg-muted'
              )}
            >
              <Monitor className="h-6 w-6" />
              <span className="text-sm font-medium">System</span>
            </button>
          </div>
          <HelpText>
            System theme will automatically match your operating system's preference
          </HelpText>
        </FormField>

        {/* Accent Color */}
        <FormField>
          <Label description="Choose your preferred accent color">
            Accent Color
          </Label>
          <div className="flex flex-wrap gap-3">
            {accentColors.map((color) => (
              <button
                key={color.value}
                onClick={() => handleAccentColorChange(color.value)}
                className={cn(
                  'flex h-12 w-12 items-center justify-center rounded-lg border-2 transition-all',
                  accentColor === color.value
                    ? 'border-foreground scale-110'
                    : 'border-transparent hover:scale-105'
                )}
                title={color.name}
              >
                <div className={cn('h-8 w-8 rounded-md', color.class)} />
              </button>
            ))}
          </div>
          <HelpText>This will affect buttons, links, and other interactive elements</HelpText>
        </FormField>

        {/* Font Size */}
        <FormField>
          <Label description="Adjust the base font size">Font Size</Label>
          <div className="flex gap-2">
            {fontSizes.map((size) => (
              <button
                key={size.value}
                onClick={() => handleFontSizeChange(size.value)}
                className={cn(
                  'flex-1 rounded-lg border-2 px-4 py-2 text-sm font-medium transition-colors',
                  fontSize === size.value
                    ? 'border-primary bg-primary/5 text-primary'
                    : 'border-border hover:bg-muted'
                )}
              >
                {size.label}
              </button>
            ))}
          </div>
        </FormField>

        {/* Compact Mode */}
        <FormField>
          <Switch
            id="compact-mode"
            checked={compactMode}
            onChange={handleCompactModeChange}
            label="Compact mode"
            description="Reduce spacing and padding throughout the interface"
          />
        </FormField>

        {/* Animations */}
        <FormField>
          <Switch
            id="show-animations"
            checked={showAnimations}
            onChange={handleShowAnimationsChange}
            label="Show animations"
            description="Enable transitions and animations (disable for better performance)"
          />
        </FormField>

        {/* Preview */}
        <div className="rounded-lg border border-border bg-card p-6">
          <h3 className="mb-4 font-semibold text-foreground">Preview</h3>
          <div className="space-y-3">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-md bg-primary" />
              <div className="flex-1">
                <div className="h-4 w-3/4 rounded bg-foreground/10" />
                <div className="mt-2 h-3 w-1/2 rounded bg-foreground/5" />
              </div>
            </div>
            <div className="flex gap-2">
              <div className="h-8 flex-1 rounded-md bg-primary/20" />
              <div className="h-8 flex-1 rounded-md bg-muted" />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
