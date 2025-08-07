import React, { useState, useEffect } from 'react';
import {
  Card, CardContent, CardHeader, CardTitle,
  Button, Input, Label, Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
  Alert, AlertDescription, Badge
} from '@/components/ui';
import { Calculator, TrendingUp, TrendingDown, Save, RotateCcw } from 'lucide-react';

interface SimulationScenario {
  id: string;
  name: string;
  changes: ResourceChange[];
  projectedCost: number;
  currentCost: number;
  savings: number;
  savingsPercent: number;
}

interface ResourceChange {
  podName: string;
  containerName: string;
  cpuRequest: number;
  cpuLimit: number;
  memoryRequest: number;
  memoryLimit: number;
  replicas: number;
}

const CostSimulator: React.FC<{ namespace: string }> = ({ namespace }) => {
  const [scenarios, setScenarios] = useState<SimulationScenario[]>([]);
  const [currentScenario, setCurrentScenario] = useState<SimulationScenario | null>(null);
  const [currentCosts, setCurrentCosts] = useState<number>(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load current costs on component mount
  useEffect(() => {
    fetchCurrentCosts();
  }, [namespace]);

  const fetchCurrentCosts = async () => {
    try {
      const response = await fetch(`/api/costs/namespace/${namespace}`);
      if (response.ok) {
        const data = await response.json();
        setCurrentCosts(data.summary.total);
      }
    } catch (err) {
      setError('Failed to fetch current costs');
    }
  };

  const createNewScenario = () => {
    const newScenario: SimulationScenario = {
      id: Date.now().toString(),
      name: `Scenario ${scenarios.length + 1}`,
      changes: [],
      projectedCost: 0,
      currentCost: currentCosts,
      savings: 0,
      savingsPercent: 0
    };
    setScenarios([...scenarios, newScenario]);
    setCurrentScenario(newScenario);
  };

  const addResourceChange = () => {
    if (!currentScenario) return;

    const newChange: ResourceChange = {
      podName: '',
      containerName: '',
      cpuRequest: 0,
      cpuLimit: 0,
      memoryRequest: 0,
      memoryLimit: 0,
      replicas: 1
    };

    const updatedScenario = {
      ...currentScenario,
      changes: [...currentScenario.changes, newChange]
    };

    setCurrentScenario(updatedScenario);
    updateScenarios(updatedScenario);
  };

  const updateResourceChange = (index: number, field: keyof ResourceChange, value: any) => {
    if (!currentScenario) return;

    const updatedChanges = [...currentScenario.changes];
    updatedChanges[index] = { ...updatedChanges[index], [field]: value };

    const updatedScenario = {
      ...currentScenario,
      changes: updatedChanges
    };

    setCurrentScenario(updatedScenario);
    updateScenarios(updatedScenario);
  };

  const removeResourceChange = (index: number) => {
    if (!currentScenario) return;

    const updatedChanges = currentScenario.changes.filter((_, i) => i !== index);
    const updatedScenario = {
      ...currentScenario,
      changes: updatedChanges
    };

    setCurrentScenario(updatedScenario);
    updateScenarios(updatedScenario);
  };

  const updateScenarios = (updatedScenario: SimulationScenario) => {
    const updatedScenarios = scenarios.map(s => 
      s.id === updatedScenario.id ? updatedScenario : s
    );
    setScenarios(updatedScenarios);
  };

  const simulateScenario = async (scenario: SimulationScenario) => {
    if (scenario.changes.length === 0) return;

    setLoading(true);
    setError(null);

    try {
      const response = await fetch('/api/simulate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          namespace: namespace,
          changes: scenario.changes,
          period: 'monthly'
        })
      });

      if (response.ok) {
        const result = await response.json();
        
        const updatedScenario = {
          ...scenario,
          projectedCost: result.projected_cost,
          savings: result.savings,
          savingsPercent: result.savings_percent
        };

        setCurrentScenario(updatedScenario);
        updateScenarios(updatedScenario);
      } else {
        setError('Failed to simulate scenario');
      }
    } catch (err) {
      setError('Failed to simulate scenario');
    } finally {
      setLoading(false);
    }
  };

  const saveScenario = (scenario: SimulationScenario) => {
    // Save scenario to localStorage for persistence
    const savedScenarios = JSON.parse(localStorage.getItem('costSimulatorScenarios') || '[]');
    const updatedScenarios = savedScenarios.filter((s: any) => s.id !== scenario.id);
    updatedScenarios.push(scenario);
    localStorage.setItem('costSimulatorScenarios', JSON.stringify(updatedScenarios));
  };

  const loadSavedScenarios = () => {
    const savedScenarios = JSON.parse(localStorage.getItem('costSimulatorScenarios') || '[]');
    setScenarios(savedScenarios);
  };

  useEffect(() => {
    loadSavedScenarios();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Cost Simulator</h2>
        <div className="flex gap-2">
          <Button onClick={createNewScenario} variant="outline">
            <Calculator className="mr-2 h-4 w-4" />
            New Scenario
          </Button>
          <Button onClick={loadSavedScenarios} variant="outline">
            <RotateCcw className="mr-2 h-4 w-4" />
            Load Saved
          </Button>
        </div>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Scenario List */}
        <Card>
          <CardHeader>
            <CardTitle>Scenarios</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {scenarios.map((scenario) => (
                <div
                  key={scenario.id}
                  className={`p-3 border rounded-lg cursor-pointer transition-colors ${
                    currentScenario?.id === scenario.id ? 'border-blue-500 bg-blue-50' : 'border-gray-200'
                  }`}
                  onClick={() => setCurrentScenario(scenario)}
                >
                  <div className="flex justify-between items-center">
                    <span className="font-medium">{scenario.name}</span>
                    {scenario.savings > 0 && (
                      <Badge variant="success" className="text-green-600">
                        +${scenario.savings.toFixed(2)}
                      </Badge>
                    )}
                  </div>
                  <div className="text-sm text-gray-500 mt-1">
                    {scenario.changes.length} changes
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        {/* Scenario Editor */}
        <div className="lg:col-span-2">
          {currentScenario ? (
            <Card>
              <CardHeader>
                <CardTitle className="flex justify-between items-center">
                  <span>{currentScenario.name}</span>
                  <div className="flex gap-2">
                    <Button
                      onClick={() => simulateScenario(currentScenario)}
                      disabled={loading || currentScenario.changes.length === 0}
                    >
                      <Calculator className="mr-2 h-4 w-4" />
                      {loading ? 'Simulating...' : 'Simulate'}
                    </Button>
                    <Button onClick={() => saveScenario(currentScenario)} variant="outline">
                      <Save className="mr-2 h-4 w-4" />
                      Save
                    </Button>
                  </div>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {/* Scenario Name */}
                  <div>
                    <Label htmlFor="scenario-name">Scenario Name</Label>
                    <Input
                      id="scenario-name"
                      value={currentScenario.name}
                      onChange={(e) => {
                        const updated = { ...currentScenario, name: e.target.value };
                        setCurrentScenario(updated);
                        updateScenarios(updated);
                      }}
                    />
                  </div>

                  {/* Resource Changes */}
                  <div>
                    <div className="flex justify-between items-center mb-4">
                      <h3 className="text-lg font-semibold">Resource Changes</h3>
                      <Button onClick={addResourceChange} variant="outline" size="sm">
                        Add Change
                      </Button>
                    </div>

                    <div className="space-y-4">
                      {currentScenario.changes.map((change, index) => (
                        <div key={index} className="border rounded-lg p-4">
                          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
                            <div>
                              <Label>Pod Name</Label>
                              <Input
                                value={change.podName}
                                onChange={(e) => updateResourceChange(index, 'podName', e.target.value)}
                                placeholder="pod-name"
                              />
                            </div>
                            <div>
                              <Label>Container</Label>
                              <Input
                                value={change.containerName}
                                onChange={(e) => updateResourceChange(index, 'containerName', e.target.value)}
                                placeholder="container-name"
                              />
                            </div>
                            <div>
                              <Label>CPU Request (m)</Label>
                              <Input
                                type="number"
                                value={change.cpuRequest}
                                onChange={(e) => updateResourceChange(index, 'cpuRequest', parseFloat(e.target.value) || 0)}
                              />
                            </div>
                            <div>
                              <Label>Memory Request (Mi)</Label>
                              <Input
                                type="number"
                                value={change.memoryRequest}
                                onChange={(e) => updateResourceChange(index, 'memoryRequest', parseFloat(e.target.value) || 0)}
                              />
                            </div>
                            <div>
                              <Label>CPU Limit (m)</Label>
                              <Input
                                type="number"
                                value={change.cpuLimit}
                                onChange={(e) => updateResourceChange(index, 'cpuLimit', parseFloat(e.target.value) || 0)}
                              />
                            </div>
                            <div>
                              <Label>Memory Limit (Mi)</Label>
                              <Input
                                type="number"
                                value={change.memoryLimit}
                                onChange={(e) => updateResourceChange(index, 'memoryLimit', parseFloat(e.target.value) || 0)}
                              />
                            </div>
                            <div>
                              <Label>Replicas</Label>
                              <Input
                                type="number"
                                value={change.replicas}
                                onChange={(e) => updateResourceChange(index, 'replicas', parseInt(e.target.value) || 1)}
                                min="1"
                              />
                            </div>
                            <div className="flex items-end">
                              <Button
                                onClick={() => removeResourceChange(index)}
                                variant="destructive"
                                size="sm"
                              >
                                Remove
                              </Button>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>

                  {/* Simulation Results */}
                  {currentScenario.projectedCost > 0 && (
                    <div className="border-t pt-4">
                      <h3 className="text-lg font-semibold mb-4">Simulation Results</h3>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div className="text-center p-4 bg-gray-50 rounded-lg">
                          <div className="text-2xl font-bold text-gray-600">
                            ${currentScenario.currentCost.toFixed(2)}
                          </div>
                          <div className="text-sm text-gray-500">Current Monthly Cost</div>
                        </div>
                        <div className="text-center p-4 bg-blue-50 rounded-lg">
                          <div className="text-2xl font-bold text-blue-600">
                            ${currentScenario.projectedCost.toFixed(2)}
                          </div>
                          <div className="text-sm text-blue-500">Projected Monthly Cost</div>
                        </div>
                        <div className="text-center p-4 bg-green-50 rounded-lg">
                          <div className="text-2xl font-bold text-green-600">
                            ${currentScenario.savings.toFixed(2)}
                          </div>
                          <div className="text-sm text-green-500">
                            Monthly Savings ({currentScenario.savingsPercent.toFixed(1)}%)
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardContent className="text-center py-12">
                <Calculator className="mx-auto h-12 w-12 text-gray-400 mb-4" />
                <h3 className="text-lg font-medium text-gray-900 mb-2">No Scenario Selected</h3>
                <p className="text-gray-500 mb-4">
                  Create a new scenario or select an existing one to start simulating cost changes.
                </p>
                <Button onClick={createNewScenario}>Create New Scenario</Button>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
};

export default CostSimulator; 