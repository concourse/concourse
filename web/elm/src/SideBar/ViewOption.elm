module SideBar.ViewOption exposing (viewOption)

import Assets
import HoverState
import Message.Message exposing (DomID(..), Message(..))
import Routes
import SideBar.Styles as Styles
import SideBar.ViewOptionType exposing (ViewOption(..))
import SideBar.Views as Views


viewOption :
    { a
        | hovered : HoverState.HoverState
        , viewOption : ViewOption
    }
    -> ViewOption
    -> Views.ViewOption
viewOption session v =
    let
        isCurrent =
            session.viewOption == v

        isHovered =
            HoverState.isHovered (SideBarViewOption v) session.hovered
    in
    { icon =
        { asset = icon v
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
        , text = title v
        }
    , background =
        if isCurrent then
            Styles.Dark

        else if isHovered then
            Styles.Light

        else
            Styles.Invisible
    , domID = SideBarViewOption v
    }


icon : ViewOption -> Assets.Asset
icon v =
    case v of
        ViewNonArchivedPipelines ->
            Assets.BreadcrumbIcon Assets.PipelineComponent

        ViewArchivedPipelines ->
            Assets.ArchivedPipelineIcon


title : ViewOption -> String
title v =
    case v of
        ViewNonArchivedPipelines ->
            "Active/Paused"

        ViewArchivedPipelines ->
            "Archived"
