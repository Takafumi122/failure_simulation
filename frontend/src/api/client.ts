import { CompareResponse, ScenariosResponse, SimulationRequest } from '../types/simulation';

const API_BASE = import.meta.env.VITE_API_BASE_URL || '';

export async function fetchScenarios(): Promise<ScenariosResponse> {
  const res = await fetch(`${API_BASE}/api/scenarios`);
  if (!res.ok) {
    throw new Error(`Failed to load scenarios (HTTP ${res.status})`);
  }
  return res.json();
}

export async function runSimulationCompare(req: SimulationRequest): Promise<CompareResponse> {
  const res = await fetch(`${API_BASE}/api/simulations/compare`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    let errText = `HTTP ${res.status}`;
    try {
      const errJson = await res.json();
      errText = errJson.details || errJson.error || errText;
    } catch {
      // ignore json parse error
    }
    throw new Error(errText);
  }
  return res.json();
}
