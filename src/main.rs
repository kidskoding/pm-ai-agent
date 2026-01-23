use rig::{client::{CompletionClient, ProviderClient}, completion::Prompt, providers::{gemini}};

#[tokio::main]
async fn main() -> anyhow::Result<(), anyhow::Error> {
    let client = gemini::Client::from_env();

    let comedian_agent = client
        .agent("gemini-2.5-pro")
        .preamble("You are a comedian here to entertain using humor and jokes!")
        .build();

    let response = comedian_agent
        .prompt("Entertain me!").await?;

    println!("{}", response);

    Ok(())
}