import type { InputHTMLAttributes, ReactNode } from "react";

type FieldProps = {
  label: string;
  name: string;
  error?: string;
  hint?: string;
  children: ReactNode;
};

export function Field({ label, name, error, hint, children }: FieldProps) {
  return (
    <div>
      <label className="label" htmlFor={name}>
        {label}
      </label>
      {children}
      {error ? (
        <p className="field-error" id={`${name}-error`} role="alert">
          {error}
        </p>
      ) : hint ? (
        <p className="hint">{hint}</p>
      ) : null}
    </div>
  );
}

type TextInputProps = InputHTMLAttributes<HTMLInputElement> & {
  error?: string;
};

export function TextInput({ error, ...props }: TextInputProps) {
  return (
    <input
      className="input"
      aria-invalid={error ? true : undefined}
      aria-describedby={error && props.name ? `${props.name}-error` : undefined}
      {...props}
    />
  );
}

export function FormError({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <p className="field-error" role="alert">
      {message}
    </p>
  );
}

export function PageSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className="grid gap-3" aria-label="Memuat">
      {Array.from({ length: lines }).map((_, i) => (
        <div key={i} className="skeleton h-12 w-full" />
      ))}
    </div>
  );
}
