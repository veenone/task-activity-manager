// The Components view's reads and writes (#605). Every write is journaled
// and Commit sends it, as for releases.

import * as App from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import type { Issue } from "./issues";
import type { Component } from "./catalog";

// ComponentSummary is one component with how many cached issues carry it.
export interface ComponentSummary extends Component {
  issues: number;
}

// ComponentFields is what the form sends, and what a component_create and
// both halves of a component_edit row hold. leadName is a username.
export interface ComponentFields {
  name: string;
  description: string;
  leadName: string;
  leadDisplayName: string;
  assigneeType: string;
}

// ComponentDelete is a component_delete row's payload.
export interface ComponentDelete {
  name: string;
  moveTo: number;
  moveToName: string;
}

// The journal entities of a component, mirroring issuerepo's.
export const ENTITY_COMPONENT_CREATE = "component_create";
export const ENTITY_COMPONENT_EDIT = "component_edit";
export const ENTITY_COMPONENT_DELETE = "component_delete";
export const COMPONENT_ENTITIES: string[] = [ENTITY_COMPONENT_CREATE, ENTITY_COMPONENT_EDIT, ENTITY_COMPONENT_DELETE];

export const ListComponentSummaries: (profileId: string) => Promise<ComponentSummary[]> =
  App.ListComponentSummaries as unknown as (profileId: string) => Promise<ComponentSummary[]>;

export const ComponentIssues: (profileId: string, id: number) => Promise<Issue[]> =
  App.ComponentIssues as unknown as (profileId: string, id: number) => Promise<Issue[]>;

export function CreateComponent(profileId: string, f: ComponentFields): Promise<Component> {
  return App.CreateComponent(profileId, main.ComponentForm.createFrom(f)) as unknown as Promise<Component>;
}

export function EditComponent(profileId: string, id: number, f: ComponentFields): Promise<void> {
  return App.EditComponent(profileId, id, main.ComponentForm.createFrom(f));
}

export const DeleteComponent: (profileId: string, id: number, moveTo: number) => Promise<void> = App.DeleteComponent;
