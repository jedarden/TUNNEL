import { cn } from '@/lib/utils'
import { AlertCircle } from 'lucide-react'

interface FormFieldProps {
  children: React.ReactNode
  className?: string
}

export function FormField({ children, className }: FormFieldProps) {
  return (
    <div className={cn('space-y-2', className)}>
      {children}
    </div>
  )
}

interface LabelProps {
  htmlFor?: string
  children: React.ReactNode
  required?: boolean
  description?: string
  className?: string
}

export function Label({
  htmlFor,
  children,
  required = false,
  description,
  className,
}: LabelProps) {
  return (
    <div className="space-y-1">
      <label
        htmlFor={htmlFor}
        className={cn(
          'block text-sm font-medium text-foreground',
          className
        )}
      >
        {children}
        {required && <span className="ml-1 text-red-500">*</span>}
      </label>
      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}
    </div>
  )
}

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  error?: string
}

export function Input({
  className,
  error,
  ...props
}: InputProps) {
  return (
    <div className="w-full">
      <input
        className={cn(
          'flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm ring-offset-background transition-colors',
          'file:border-0 file:bg-transparent file:text-sm file:font-medium',
          'placeholder:text-muted-foreground',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2',
          'disabled:cursor-not-allowed disabled:opacity-50',
          error && 'border-red-500 focus-visible:ring-red-500',
          className
        )}
        {...props}
      />
      {error && <ErrorMessage>{error}</ErrorMessage>}
    </div>
  )
}

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: string
}

export function Textarea({
  className,
  error,
  ...props
}: TextareaProps) {
  return (
    <div className="w-full">
      <textarea
        className={cn(
          'flex min-h-[80px] w-full rounded-md border border-border bg-background px-3 py-2 text-sm ring-offset-background transition-colors',
          'placeholder:text-muted-foreground',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2',
          'disabled:cursor-not-allowed disabled:opacity-50',
          error && 'border-red-500 focus-visible:ring-red-500',
          className
        )}
        {...props}
      />
      {error && <ErrorMessage>{error}</ErrorMessage>}
    </div>
  )
}

interface ErrorMessageProps {
  children: React.ReactNode
  className?: string
}

export function ErrorMessage({ children, className }: ErrorMessageProps) {
  return (
    <div className={cn('mt-1 flex items-center gap-1 text-xs text-red-500', className)}>
      <AlertCircle className="h-3 w-3" />
      <span>{children}</span>
    </div>
  )
}

interface HelpTextProps {
  children: React.ReactNode
  className?: string
}

export function HelpText({ children, className }: HelpTextProps) {
  return (
    <p className={cn('text-xs text-muted-foreground', className)}>
      {children}
    </p>
  )
}

interface FormActionsProps {
  children: React.ReactNode
  className?: string
}

export function FormActions({ children, className }: FormActionsProps) {
  return (
    <div className={cn('flex items-center justify-end gap-3', className)}>
      {children}
    </div>
  )
}
