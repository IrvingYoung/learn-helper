import type { Topic, TopicDetail, Exercise } from "../types";

const BASE = "/api";

export async function fetchTopics(): Promise<Topic[]> {
  const res = await fetch(`${BASE}/topics`);
  if (!res.ok) throw new Error("Failed to fetch topics");
  const data = await res.json();
  return data.topics ?? [];
}

export async function fetchTopicBySlug(slug: string): Promise<TopicDetail> {
  const res = await fetch(`${BASE}/topics/${slug}`);
  if (!res.ok) throw new Error(`Failed to fetch topic: ${slug}`);
  return res.json();
}

export async function fetchExercises(params?: { topic_id?: number; difficulty?: string }): Promise<Exercise[]> {
  const query = new URLSearchParams();
  if (params?.topic_id) query.set("topic_id", String(params.topic_id));
  if (params?.difficulty) query.set("difficulty", params.difficulty);
  const res = await fetch(`${BASE}/exercises?${query.toString()}`);
  if (!res.ok) throw new Error("Failed to fetch exercises");
  const data = await res.json();
  return data.exercises ?? [];
}

export async function fetchExerciseById(id: number): Promise<Exercise> {
  const res = await fetch(`${BASE}/exercises/${id}`);
  if (!res.ok) throw new Error(`Failed to fetch exercise: ${id}`);
  return res.json();
}
