module SideBar.Pipeline exposing (pipeline, text)

import Assets
import Concourse
import Dict
import Favorites
import HoverState
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Routes
import Set exposing (Set)
import SideBar.Styles as Styles
import SideBar.Views as Views


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
        , pipelineInstanceVars : Concourse.InstanceVars
    }


pipeline :
    { a
        | hovered : HoverState.HoverState
        , currentPipeline : Maybe (PipelineScoped b)
        , favoritedPipelines : Set Int
        , isFavoritesSection : Bool
    }
    -> Concourse.Pipeline
    -> Views.Pipeline
pipeline params p =
    let
        isCurrent =
            case params.currentPipeline of
                Just cp ->
                    (cp.pipelineName == p.name)
                        && (cp.teamName == p.teamName)
                        && (cp.pipelineInstanceVars == p.instanceVars)

                Nothing ->
                    False

        pipelineId =
            Concourse.toPipelineId p

        domID =
            SideBarPipeline
                (if params.isFavoritesSection then
                    FavoritesSection

                 else
                    AllPipelinesSection
                )
                p.id

        isHovered =
            HoverState.isHovered domID params.hovered

        isFavorited =
            Favorites.isPipelineFavorited params p
    in
    { icon =
        if p.archived then
            Assets.ArchivedPipelineIcon

        else if isHovered then
            Assets.PipelineIconWhite

        else if isCurrent then
            Assets.PipelineIconLightGrey

        else
            Assets.PipelineIconGrey
    , name =
        { color =
            if isHovered then
                Styles.White

            else if isCurrent then
                Styles.LightGrey

            else
                Styles.Grey
        , text = text p
        , weight =
            if isCurrent || isHovered then
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
    , domID = domID
    , starIcon =
        { filled = isFavorited
        , isBright = isHovered || isCurrent
        }
    , id = pipelineId
    , databaseID = p.id
    }


text : Concourse.Pipeline -> String
text p =
    let
        instanceVarsText =
            if Dict.isEmpty p.instanceVars then
                ""

            else
                "/"
                    ++ (p.instanceVars
                            |> Dict.toList
                            |> List.concatMap
                                (\( k, v ) ->
                                    Concourse.flattenJson k v
                                        |> List.map (\( key, val ) -> key ++ ":" ++ val)
                                )
                            |> String.join ","
                       )
    in
    p.name ++ instanceVarsText
