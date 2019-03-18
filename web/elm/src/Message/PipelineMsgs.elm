module Message.PipelineMsgs exposing (Msg(..))

import Concourse
import Message.TopBarMsgs


type Msg
    = PipelineIdentifierFetched Concourse.PipelineIdentifier
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
    | FromTopBar Message.TopBarMsgs.Msg
