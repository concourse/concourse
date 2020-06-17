module SideBar.Pipeline exposing (pipeline)

import Assets
import Concourse
import HoverState
import Message.Message exposing (DomID(..), Message(..))
import Routes
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
    }


pipeline :
    { a
        | hovered : HoverState.HoverState
        , currentPipeline : Maybe (PipelineScoped b)
        , favoritedPipelines : List Int
    }
    -> Concourse.Pipeline
    -> Views.Pipeline
pipeline session p =
    let
        isCurrent =
            case session.currentPipeline of
                Just cp ->
                    cp.pipelineName == p.name && cp.teamName == p.teamName

                Nothing ->
                    False

        pipelineId =
            { pipelineName = p.name, teamName = p.teamName }

        isHovered =
            HoverState.isHovered (SideBarPipeline pipelineId) session.hovered
    in
    { icon =
        { asset =
            if p.archived then
                Assets.ArchivedPipelineIcon

            else
                Assets.BreadcrumbIcon Assets.PipelineComponent
        , opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        }
    , name =
        { opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        , text = p.name
        , weight =
            if isCurrent then
                Styles.Bold

            else
                Styles.Default
        }
    , background =
        if isCurrent then
            Styles.Dark

        else if isHovered then
            Styles.Light

        else
            Styles.Invisible
    , href =
        Routes.toString <|
            Routes.Pipeline { id = pipelineId, groups = [] }
    , domID = SideBarPipeline pipelineId
    , favIcon =
        { opacity =
            if isCurrent || isHovered then
                Styles.Bright

            else
                Styles.Dim
        , filled = List.member p.id session.favoritedPipelines
        }
    , id = p.id
    }
