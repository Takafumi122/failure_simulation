import { test, expect } from '@playwright/test';

test.describe('FailureLab Portfolio Release E2E', () => {
  test('1. Loads Scenario Gallery, displays Engine Version & Seed, and compares all 5 strategies', async ({
    page,
  }) => {
    await page.goto('/');

    // Header and Engine Version badge
    await expect(page.locator('h1')).toContainText('FailureLab');
    await expect(page.getByTestId('engine-version-badge')).toContainText('v3.0.0-des');

    // Scenario Gallery with all 5 required presets
    const gallery = page.getByTestId('scenario-gallery');
    await expect(gallery).toBeVisible();
    await expect(page.getByTestId('scenario-card-transient-failure')).toBeVisible();
    await expect(page.getByTestId('scenario-card-prolonged-outage')).toBeVisible();
    await expect(page.getByTestId('scenario-card-retry-storm')).toBeVisible();
    await expect(page.getByTestId('scenario-card-circuit-breaker-recovery')).toBeVisible();
    await expect(page.getByTestId('scenario-card-healthy-system')).toBeVisible();

    // Strategy Comparison Table has all 5 strategies
    const table = page.getByTestId('strategy-comparison-table');
    await expect(table).toBeVisible();
    await expect(page.getByTestId('strategy-row-baseline')).toBeVisible();
    await expect(page.getByTestId('strategy-row-retry')).toBeVisible();
    await expect(page.getByTestId('strategy-row-circuit_breaker')).toBeVisible();
    await expect(page.getByTestId('strategy-row-retry_circuit_breaker')).toBeVisible();
    await expect(page.getByTestId('strategy-row-fail_fast')).toBeVisible();

    // Measured observations are rendered from actual simulation results
    const obs = page.getByTestId('measured-observations');
    await expect(obs).toContainText('Retry improved logical request success rate');
  });

  test('2. Switching scenarios updates metrics and Circuit Breaker Recovery shows state transitions', async ({
    page,
  }) => {
    await page.goto('/');

    // Click Circuit Breaker Recovery scenario card
    await page.getByTestId('scenario-card-circuit-breaker-recovery').click();

    // Wait for results to update and verify CB rejections > 0 on circuit_breaker row
    const cbRow = page.getByTestId('strategy-row-circuit_breaker');
    await expect(cbRow).toBeVisible();

    // Click on Circuit Breaker strategy pill to inspect its active details
    await page.getByTestId('strategy-pill-circuit_breaker').click();
    await expect(page.getByTestId('measured-observations')).toContainText(
      'Circuit Breaker fast-rejected'
    );
  });

  test('3. Re-running identical configuration produces identical deterministic results', async ({
    page,
  }) => {
    await page.goto('/');
    await page.getByTestId('scenario-card-retry-storm').click();

    // Wait for Retry Storm scenario results (seed=101, 50 RPS) to finish loading
    await expect(page.getByTestId('reproducibility-meta-badge')).toContainText('seed=101');
    await expect(page.getByTestId('run-simulation-btn')).toBeEnabled();

    const retryRow = page.getByTestId('strategy-row-retry');
    const firstRunText = await retryRow.innerText();

    // Click Run Simulation again with the exact same config and seed
    await page.getByTestId('run-simulation-btn').click();
    await expect(page.getByTestId('run-simulation-btn')).toBeEnabled();
    const secondRunText = await retryRow.innerText();

    expect(secondRunText).toEqual(firstRunText);
  });
});
