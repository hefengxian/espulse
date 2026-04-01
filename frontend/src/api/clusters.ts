
export interface Cluster {
  id: string;
  name: string;
  hosts: string[]; // Change to string array to match backend
  auth_type: string;
  username?: string;
  password?: string;
  api_key?: string;
  color: string;
  notes: string;
  created_at?: string;
  updated_at?: string;
}

export const clusterApi = {
  async list(): Promise<Cluster[]> {
    const response = await fetch('/api/clusters');
    if (!response.ok) throw new Error('Failed to fetch clusters');
    return response.json();
  },

  async get(id: string): Promise<Cluster> {
    const response = await fetch(`/api/clusters/${id}`);
    if (!response.ok) throw new Error('Failed to fetch cluster');
    return response.json();
  },

  async create(cluster: Partial<Cluster>): Promise<Cluster> {
    const response = await fetch('/api/clusters', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cluster),
    });
    if (!response.ok) throw new Error('Failed to create cluster');
    return response.json();
  },

  async delete(id: string): Promise<void> {
    const response = await fetch(`/api/clusters/${id}`, {
      method: 'DELETE',
    });
    if (!response.ok) throw new Error('Failed to delete cluster');
  }
};
