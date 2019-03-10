module Pipeline.Msgs exposing (Msg(..))

import Concourse
import TopBar.Msgs


type Msg
    = PipelineIdentifierFetched Concourse.PipelineIdentifier
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
    | FromTopBar TopBar.Msgs.Msg
