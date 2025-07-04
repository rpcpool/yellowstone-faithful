import React from 'react';
import { render, screen } from '@testing-library/react';
import { Badge } from '@/components/ui/badge';

describe('Badge component', () => {
  it('should render with default variant', () => {
    render(<Badge>Default Badge</Badge>);
    const badge = screen.getByText('Default Badge');
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveAttribute('data-slot', 'badge');
  });

  it('should render with secondary variant', () => {
    render(<Badge variant="secondary">Secondary Badge</Badge>);
    const badge = screen.getByText('Secondary Badge');
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain('bg-secondary');
  });

  it('should render with destructive variant', () => {
    render(<Badge variant="destructive">Destructive Badge</Badge>);
    const badge = screen.getByText('Destructive Badge');
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain('bg-destructive');
  });

  it('should render with outline variant', () => {
    render(<Badge variant="outline">Outline Badge</Badge>);
    const badge = screen.getByText('Outline Badge');
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain('text-foreground');
  });

  it('should apply custom className', () => {
    render(<Badge className="custom-class">Custom Badge</Badge>);
    const badge = screen.getByText('Custom Badge');
    expect(badge).toHaveClass('custom-class');
  });

  it('should render as child when asChild is true', () => {
    render(
      <Badge asChild>
        <a href="/test">Link Badge</a>
      </Badge>
    );
    const link = screen.getByRole('link', { name: 'Link Badge' });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/test');
    expect(link).toHaveAttribute('data-slot', 'badge');
  });

  it('should render with children elements', () => {
    render(
      <Badge>
        <svg />
        <span>Badge with Icon</span>
      </Badge>
    );
    const badge = screen.getByText('Badge with Icon');
    expect(badge.parentElement).toBeInTheDocument();
    expect(badge.parentElement?.querySelector('svg')).toBeInTheDocument();
  });

  it('should pass through additional props', () => {
    render(
      <Badge data-testid="test-badge" aria-label="Test Badge">
        Badge
      </Badge>
    );
    const badge = screen.getByTestId('test-badge');
    expect(badge).toHaveAttribute('aria-label', 'Test Badge');
  });
});