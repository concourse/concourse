module SideBar.Team exposing (team)

import Concourse
import Message.Message exposing (DomID(..), Message(..))
import SideBar.Pipeline as Pipeline
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


team :
    { a
        | hovered : Maybe DomID
        , pipelines : List Concourse.Pipeline
        , currentPipeline : Maybe (PipelineScoped b)
    }
    -> { name : String, isExpanded : Bool }
    -> Views.Team
team session t =
    let
        isHovered =
            session.hovered == Just (SideBarTeam t.name)

        isCurrent =
            (session.currentPipeline |> Maybe.map .teamName) == Just t.name
    in
    { icon =
        if isCurrent then
            Styles.Bright

        else if isHovered || t.isExpanded then
            Styles.GreyedOut

        else
            Styles.Dim
    , arrow =
        { opacity =
            if isCurrent then
                Styles.Bright

            else if t.isExpanded then
                Styles.GreyedOut

            else
                Styles.Dim
        , icon =
            if t.isExpanded then
                Styles.Down

            else
                Styles.Right
        }
    , name =
        { text = t.name
        , opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.GreyedOut
        , rectangle =
            if isHovered then
                Styles.GreyWithLightBorder

            else
                Styles.TeamInvisible
        }
    , isExpanded = t.isExpanded
    , pipelines = List.map (Pipeline.pipeline session) session.pipelines
    }
