import { Task } from "@/lib/interfaces/task";
import fetchEpochCidsTask from "./fetch_epoch_cids";
import { getGsfaIndexTask } from "./get_gsfa_index";
import { getStandardIndexesTask } from "./get_standard_indexes";
import { refreshEpochTask } from "./refresh_epoch";
import { refreshSourceTask } from "./refresh_source";

function allTasks(): Task[] {
  return [
    fetchEpochCidsTask,
    getGsfaIndexTask,
    getStandardIndexesTask,
    refreshSourceTask,
    refreshEpochTask,
  ];
}

function getTask(name: string): Task | undefined {
  return allTasks().find((task) => task.name === name);
}

export { allTasks, fetchEpochCidsTask, getGsfaIndexTask, getStandardIndexesTask, getTask, refreshEpochTask, refreshSourceTask };

