use clap::{Parser, Subcommand};

#[derive(Parser)]
#[command(name = "jot")]
#[command(about = "quicky memo")]
pub struct Cli {
    #[command(subcommand)]
    command: CliCommand,
}

#[derive(Subcommand)]
pub enum CliCommand {
    New,
    List,
}
