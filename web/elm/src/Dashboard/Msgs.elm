module Dashboard.Msgs exposing (Msg(..))

import Concourse.Cli as Cli
import Dashboard.Models as Models
import TopBar.Msgs


type Msg
    = DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
    | TogglePipelinePaused Models.Pipeline
    | PipelineButtonHover (Maybe Models.Pipeline)
    | CliHover (Maybe Cli.Cli)
    | TopCliHover (Maybe Cli.Cli)
    | FromTopBar TopBar.Msgs.Msg
