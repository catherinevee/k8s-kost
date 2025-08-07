import React, { useState, useEffect } from 'react';
import {
  LineChart, Line, AreaChart, Area, BarChart, Bar,
  PieChart, Pie, Cell, XAxis, YAxis, CartesianGrid,
  Tooltip, Legend, ResponsiveContainer
} from 'recharts';
import {
  Card, CardContent, CardHeader, CardTitle,
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  Button, Badge, Alert, AlertDescription,
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
  Tabs, TabsContent, TabsList, TabsTrigger
} from '@/components/ui';
import { Download, TrendingDown, TrendingUp, AlertTriangle, CheckCircle, DollarSign, Activity } from 'lucide-react';

interface CostData {
  namespace: string;
  period: string;
  costs: DailyCost[];
  summary: {
    total: number;
    average_daily: number;
    projected_monthly: number;
  };
  breakdown: {
    compute: number;
    storage: number;
    network: number;
    other: number;
  };
}

interface DailyCost {
  date: string;
  compute: number;
  storage: number;
  network: number;
  other: number;
  total: number;
}

interface Recommendation {
  namespace: string;
  pod_name: string;
  container_name: string;
  resource_type: string;
  current_request: number;
  current_limit: number;
  recommended_request: number;
  recommended_limit: number;
  p50_usage: number;
  p95_usage: number;
  p99_usage: number;
  max_usage: number;
  potential_savings: number;
  confidence: number;
  reasoning: string;
  risk_level: string;
}

interface RecommendationsData {
  namespace: string;
  recommendations: Record<string, Recommendation[]>;
  total_savings: number;
  annual_savings: number;
  patches: string[];
  apply_command: string;
  confidence_score: number;
}

const CostDashboard: React.FC = () => {
  const [namespace, setNamespace] = useState('default');
  const [period, setPeriod] = useState('30d');
  const [costs, setCosts] = useState<CostData | null>(null);
  const [recommendations, setRecommendations] = useState<RecommendationsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedPod, setSelectedPod] = useState<string | null>(null);

  useEffect(() => {
    fetchCosts();
    fetchRecommendations();
  }, [namespace, period]);

  const fetchCosts = async () => {
    setLoading(true);
    try {
      const response = await fetch(
        `/api/costs/namespace/${namespace}?period=${period}`
      );
      const data = await response.json();
      setCosts(data);
    } catch (error) {
      console.error('Failed to fetch costs:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchRecommendations = async () => {
    try {
      const response = await fetch(
        `/api/recommendations/${namespace}`
      );
      const data = await response.json();
      setRecommendations(data);
    } catch (error) {
      console.error('Failed to fetch recommendations:', error);
    }
  };

  const applyRecommendation = async (recommendation: Recommendation) => {
    const confirmed = window.confirm(
      `Apply recommendation for ${recommendation.pod_name}? ` +
      `This will save approximately $${recommendation.potential_savings.toFixed(2)}/month.`
    );

    if (confirmed) {
      try {
        const response = await fetch('/api/recommendations/apply', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            namespace: recommendation.namespace,
            pod_name: recommendation.pod_name,
            container_name: recommendation.container_name,
            resource_type: recommendation.resource_type,
            action: 'apply'
          })
        });

        if (response.ok) {
          alert('Recommendation applied successfully!');
          fetchRecommendations();
        }
      } catch (error) {
        alert('Failed to apply recommendation: ' + (error as Error).message);
      }
    }
  };

  const exportReport = async (format: string) => {
    const response = await fetch(
      `/api/export?namespace=${namespace}&format=${format}`
    );
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `cost-report-${namespace}.${format}`;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
  };

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2
    }).format(value);
  };

  const formatPercentage = (value: number) => {
    return `${(value * 100).toFixed(1)}%`;
  };

  const formatResourceValue = (resourceType: string, value: number) => {
    if (resourceType === 'CPU') {
      return `${value.toFixed(0)}m`;
    } else {
      return `${(value / 1024 / 1024).toFixed(0)}Mi`;
    }
  };

  const COLORS = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884D8'];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-32 w-32 border-b-2 border-blue-600"></div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold">Kubernetes Cost Optimizer</h1>
          <p className="text-gray-600">Real-time cost analysis and optimization recommendations</p>
        </div>
        <div className="flex gap-4">
          <Select value={namespace} onValueChange={setNamespace}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Select namespace" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="default">default</SelectItem>
              <SelectItem value="production">production</SelectItem>
              <SelectItem value="staging">staging</SelectItem>
              <SelectItem value="development">development</SelectItem>
            </SelectContent>
          </Select>

          <Select value={period} onValueChange={setPeriod}>
            <SelectTrigger className="w-[120px]">
              <SelectValue placeholder="Period" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="24h">24 hours</SelectItem>
              <SelectItem value="7d">7 days</SelectItem>
              <SelectItem value="30d">30 days</SelectItem>
            </SelectContent>
          </Select>

          <Button onClick={() => exportReport('pdf')}>
            <Download className="mr-2 h-4 w-4" />
            Export PDF
          </Button>
        </div>
      </div>

      {/* Summary Cards */}
      {costs && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Total Cost
              </CardTitle>
              <DollarSign className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatCurrency(costs.summary.total)}
              </div>
              <p className="text-xs text-muted-foreground">
                {period} period
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Daily Average
              </CardTitle>
              <Activity className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatCurrency(costs.summary.average_daily)}
              </div>
              <p className="text-xs text-muted-foreground">
                Per day
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Monthly Projection
              </CardTitle>
              <TrendingUp className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">
                {formatCurrency(costs.summary.projected_monthly)}
              </div>
              <p className="text-xs text-muted-foreground">
                Based on current usage
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                Potential Savings
              </CardTitle>
              <TrendingDown className="h-4 w-4 text-green-600" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold text-green-600">
                {formatCurrency(recommendations?.total_savings || 0)}
              </div>
              <p className="text-xs text-muted-foreground">
                Monthly savings available
              </p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Main Content Tabs */}
      <Tabs defaultValue="overview" className="space-y-4">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="recommendations">Recommendations</TabsTrigger>
          <TabsTrigger value="resources">Resources</TabsTrigger>
          <TabsTrigger value="simulator">Cost Simulator</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4">
          {/* Cost Trend Chart */}
          <Card>
            <CardHeader>
              <CardTitle>Cost Trend</CardTitle>
            </CardHeader>
            <CardContent>
              <ResponsiveContainer width="100%" height={300}>
                <AreaChart data={costs?.costs || []}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="date" />
                  <YAxis />
                  <Tooltip formatter={(value) => formatCurrency(value as number)} />
                  <Legend />
                  <Area
                    type="monotone"
                    dataKey="compute"
                    stackId="1"
                    stroke="#8884d8"
                    fill="#8884d8"
                  />
                  <Area
                    type="monotone"
                    dataKey="storage"
                    stackId="1"
                    stroke="#82ca9d"
                    fill="#82ca9d"
                  />
                  <Area
                    type="monotone"
                    dataKey="network"
                    stackId="1"
                    stroke="#ffc658"
                    fill="#ffc658"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>

          {/* Cost Breakdown Pie Chart */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Card>
              <CardHeader>
                <CardTitle>Cost Breakdown</CardTitle>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <PieChart>
                    <Pie
                      data={[
                        { name: 'Compute', value: costs?.breakdown?.compute || 0 },
                        { name: 'Storage', value: costs?.breakdown?.storage || 0 },
                        { name: 'Network', value: costs?.breakdown?.network || 0 },
                        { name: 'Other', value: costs?.breakdown?.other || 0 }
                      ]}
                      cx="50%"
                      cy="50%"
                      labelLine={false}
                      label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                      outerRadius={80}
                      fill="#8884d8"
                      dataKey="value"
                    >
                      {costs?.breakdown && Object.keys(costs.breakdown).map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip formatter={(value) => formatCurrency(value as number)} />
                  </PieChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Daily Cost Trend</CardTitle>
              </CardHeader>
              <CardContent>
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={costs?.costs || []}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="date" />
                    <YAxis />
                    <Tooltip formatter={(value) => formatCurrency(value as number)} />
                    <Legend />
                    <Line
                      type="monotone"
                      dataKey="total"
                      stroke="#8884d8"
                      strokeWidth={2}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="recommendations" className="space-y-4">
          {recommendations && recommendations.recommendations && (
            <>
              <Alert>
                <AlertTriangle className="h-4 w-4" />
                <AlertDescription>
                  Found {Object.keys(recommendations.recommendations).length} pods with optimization opportunities.
                  Total potential savings: {formatCurrency(recommendations.total_savings)}/month
                </AlertDescription>
              </Alert>

              <Card>
                <CardHeader>
                  <CardTitle>Resource Optimization Recommendations</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Pod</TableHead>
                        <TableHead>Container</TableHead>
                        <TableHead>Resource</TableHead>
                        <TableHead>Current</TableHead>
                        <TableHead>Recommended</TableHead>
                        <TableHead>Savings</TableHead>
                        <TableHead>Confidence</TableHead>
                        <TableHead>Risk</TableHead>
                        <TableHead>Action</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {Object.entries(recommendations.recommendations).map(([pod, recs]) =>
                        recs.map((rec, idx) => (
                          <TableRow key={`${pod}-${idx}`}>
                            <TableCell>{rec.pod_name}</TableCell>
                            <TableCell>{rec.container_name}</TableCell>
                            <TableCell>
                              <Badge variant={rec.resource_type === 'CPU' ? 'default' : 'secondary'}>
                                {rec.resource_type}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              {formatResourceValue(rec.resource_type, rec.current_request)}
                            </TableCell>
                            <TableCell>
                              {formatResourceValue(rec.resource_type, rec.recommended_request)}
                            </TableCell>
                            <TableCell className="text-green-600">
                              {formatCurrency(rec.potential_savings)}
                            </TableCell>
                            <TableCell>
                              <Badge variant={rec.confidence > 0.8 ? 'default' : rec.confidence > 0.6 ? 'secondary' : 'destructive'}>
                                {formatPercentage(rec.confidence)}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              <Badge variant={rec.risk_level === 'LOW' ? 'default' : rec.risk_level === 'MEDIUM' ? 'secondary' : 'destructive'}>
                                {rec.risk_level}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              <Button
                                size="sm"
                                onClick={() => applyRecommendation(rec)}
                                disabled={rec.confidence < 0.7}
                              >
                                Apply
                              </Button>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Apply All Recommendations</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-4">
                    <p>
                      You can apply all high-confidence recommendations at once.
                      This will save approximately {formatCurrency(recommendations.annual_savings)} per year.
                    </p>
                    <div className="flex gap-4">
                      <Button
                        variant="default"
                        onClick={() => {
                          const blob = new Blob([recommendations.patches.join('\n---\n')], { type: 'text/yaml' });
                          const url = window.URL.createObjectURL(blob);
                          const a = document.createElement('a');
                          a.href = url;
                          a.download = `recommendations-${namespace}.yaml`;
                          document.body.appendChild(a);
                          a.click();
                          window.URL.revokeObjectURL(url);
                        }}
                      >
                        <Download className="mr-2 h-4 w-4" />
                        Download YAML
                      </Button>
                      <Button
                        variant="outline"
                        onClick={() => {
                          navigator.clipboard.writeText(recommendations.apply_command);
                          alert('Command copied to clipboard!');
                        }}
                      >
                        Copy kubectl command
                      </Button>
                    </div>
                    <pre className="bg-gray-100 p-4 rounded overflow-x-auto text-sm">
                      <code>{recommendations.apply_command}</code>
                    </pre>
                  </div>
                </CardContent>
              </Card>
            </>
          )}
        </TabsContent>

        <TabsContent value="resources" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Resource Usage</CardTitle>
            </CardHeader>
            <CardContent>
              <p>Resource usage details will be displayed here.</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="simulator" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Cost Simulator</CardTitle>
            </CardHeader>
            <CardContent>
              <p>Cost simulation tool will be displayed here.</p>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default CostDashboard; 