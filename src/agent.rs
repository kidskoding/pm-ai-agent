use std::env;

use anyhow::anyhow;
use rig::agent::Agent;
use rig::client::CompletionClient;
use rig::completion::CompletionModel;
use rig_vertexai::ClientBuilder;

pub async fn create_pm_agent() -> anyhow::Result<Agent<impl CompletionModel>> {
    dotenvy::dotenv().ok();
    
    let project_id = env::var("GCP_PROJECT_ID")
        .map_err(|_| anyhow!("GCP_PROJECT_ID is not set"))?;
    let region = env::var("VERTEX_AI_REGION")
        .unwrap_or_else(|_| "us-central1".to_string());

    let client = ClientBuilder::new()
        .with_project(&project_id)
        .with_location(&region)
        .build()
        .map_err(|e| anyhow!(e))?;

    let agent = client
        .agent("gemini-2.5-flash")
        .preamble("You are an autonomous PM. Transform ideas into User Stories.")
        .build();

    Ok(agent)
}