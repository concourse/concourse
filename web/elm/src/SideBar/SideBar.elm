module SideBar.SideBar exposing
    ( Model
    , byDatabaseId
    , byPipelineId
    , handleCallback
    , handleDelivery
    , isPipelineVisible
    , lookupPipeline
    , sideBarIcon
    , tooltip
    , update
    , view
    )

import Assets
import Concourse
import EffectTransformer exposing (ET)
import Favorites
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (id)
import Html.Events exposing (onClick, onMouseDown, onMouseEnter, onMouseLeave)
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message
    exposing
        ( DomID(..)
        , Message(..)
        , PipelinesSection(..)
        , VisibilityAction(..)
        )
import Message.Subscription exposing (Delivery(..))
import RemoteData exposing (RemoteData(..), WebData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import Set exposing (Set)
import SideBar.Pipeline as Pipeline
import SideBar.State exposing (SideBarState)
import SideBar.Styles as Styles
import SideBar.Team as Team exposing (PipelineType(..))
import SideBar.Views as Views
import Tooltip
import Views.Icon as Icon
import Views.Styles


type alias Model m =
    Tooltip.Model
        (Favorites.Model
            { m
                | expandedTeamsInAllPipelines : Set String
                , collapsedTeamsInFavorites : Set String
                , pipelines : WebData (List Concourse.Pipeline)
                , sideBarState : SideBarState
                , draggingSideBar : Bool
                , screenSize : ScreenSize.ScreenSize
                , route : Routes.Route
            }
        )


type alias PipelineScoped a =
    { a
        | teamName : String
        , pipelineName : String
        , pipelineInstanceVars : Concourse.InstanceVars
    }


update : Message -> Model m -> ( Model m, List Effects.Effect )
update message model =
    updateSidebar message model |> Favorites.update message


updateSidebar : Message -> Model m -> ( Model m, List Effects.Effect )
updateSidebar message model =
    let
        toggle element set =
            if Set.member element set then
                Set.remove element set

            else
                Set.insert element set
    in
    case message of
        Click SideBarIcon ->
            let
                oldState =
                    model.sideBarState

                newState =
                    { oldState | isOpen = not oldState.isOpen }
            in
            ( { model | sideBarState = newState }
            , [ Effects.SaveSideBarState newState ]
            )

        Click (SideBarTeam section teamName) ->
            case section of
                AllPipelinesSection ->
                    ( { model
                        | expandedTeamsInAllPipelines =
                            toggle teamName model.expandedTeamsInAllPipelines
                      }
                    , []
                    )

                FavoritesSection ->
                    ( { model
                        | collapsedTeamsInFavorites =
                            toggle teamName model.collapsedTeamsInFavorites
                      }
                    , []
                    )

        Click SideBarResizeHandle ->
            ( { model | draggingSideBar = True }, [] )

        Hover (Just domID) ->
            ( model, [ Effects.GetViewportOf domID ] )

        _ ->
            ( model, [] )


handleCallback : Callback -> ET (Model m)
handleCallback callback ( model, effects ) =
    case callback of
        AllPipelinesFetched (Ok pipelines) ->
            ( { model
                | pipelines = Success pipelines
                , expandedTeamsInAllPipelines =
                    case ( model.pipelines, curPipeline pipelines model.route ) of
                        -- First time receiving AllPipelines response
                        ( NotAsked, Just { teamName } ) ->
                            model.expandedTeamsInAllPipelines
                                |> Set.insert teamName

                        _ ->
                            model.expandedTeamsInAllPipelines
              }
            , effects
            )

        BuildFetched (Ok build) ->
            ( { model
                | expandedTeamsInAllPipelines =
                    case ( model.route, build.job, build ) of
                        -- One-off build page for a job build should still expand team in sidebar
                        ( Routes.OneOffBuild _, Just _, { teamName } ) ->
                            model.expandedTeamsInAllPipelines
                                |> Set.insert teamName

                        _ ->
                            model.expandedTeamsInAllPipelines
              }
            , effects
            )

        VisibilityChanged Hide id (Ok ()) ->
            ( updatePipeline
                (\p -> { p | public = False })
                (byPipelineId id)
                model
            , effects
            )

        VisibilityChanged Expose id (Ok ()) ->
            ( updatePipeline
                (\p -> { p | public = True })
                (byPipelineId id)
                model
            , effects
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET (Model m)
handleDelivery delivery =
    handleDeliverySidebar delivery >> Favorites.handleDelivery delivery


handleDeliverySidebar : Delivery -> ET (Model m)
handleDeliverySidebar delivery ( model, effects ) =
    case delivery of
        SideBarStateReceived (Ok state) ->
            ( { model | sideBarState = state }, effects )

        Moused pos ->
            if model.draggingSideBar then
                let
                    oldState =
                        model.sideBarState

                    newState =
                        { oldState | width = pos.x }
                in
                ( { model | sideBarState = newState }
                , effects ++ [ Effects.GetViewportOf Dashboard ]
                )

            else
                ( model, effects )

        MouseUp ->
            ( { model | draggingSideBar = False }
            , if model.draggingSideBar then
                [ Effects.SaveSideBarState model.sideBarState ]

              else
                []
            )

        _ ->
            ( model, effects )


view : Model m -> Maybe (PipelineScoped a) -> Html Message
view model currentPipeline =
    if
        model.sideBarState.isOpen
            && hasVisiblePipelines model
            && (model.screenSize /= ScreenSize.Mobile)
    then
        let
            oldState =
                model.sideBarState

            newState =
                { oldState | width = clamp 100 600 oldState.width }
        in
        Html.div
            (id "side-bar" :: Styles.sideBar newState)
            -- I'd love to use the curPipeline function instead of passing it in to view,
            -- but that doesn't work for OneOffBuilds that point to a JobBuild
            (favoritedPipelinesSection model currentPipeline
                ++ allPipelinesSection model currentPipeline
                ++ [ Html.div
                        (Styles.sideBarHandle newState
                            ++ [ onMouseDown <| Click SideBarResizeHandle ]
                        )
                        []
                   ]
            )

    else
        Html.text ""


tooltip : Model m -> Maybe Tooltip.Tooltip
tooltip model =
    let
        isSideBarClickable =
            hasVisiblePipelines model

        beyondStarOffset =
            Styles.tooltipArrowSize
                + (Styles.starPadding * 2)
                + Styles.starWidth
                - Styles.tooltipOffset
    in
    case model.hovered of
        HoverState.Tooltip (SideBarTeam _ teamName) _ ->
            Just
                { body = Html.text teamName
                , attachPosition =
                    { direction =
                        Tooltip.Right (Styles.tooltipArrowSize - Styles.tooltipOffset)
                    , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                    }
                , arrow = Just Styles.tooltipArrowSize
                , containerAttrs = Just Styles.tooltipBody
                }

        HoverState.Tooltip (SideBarPipeline _ id) _ ->
            lookupPipeline (byDatabaseId id) model
                |> Maybe.map
                    (\p ->
                        { body = Html.text <| Pipeline.regularPipelineText p
                        , attachPosition =
                            { direction = Tooltip.Right beyondStarOffset
                            , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                            }
                        , arrow = Just Styles.tooltipArrowSize
                        , containerAttrs = Just Styles.tooltipBody
                        }
                    )

        HoverState.Tooltip (SideBarInstancedPipeline _ id) _ ->
            lookupPipeline (byDatabaseId id) model
                |> Maybe.map
                    (\p ->
                        { body = Html.text <| Pipeline.instancedPipelineText p
                        , attachPosition =
                            { direction = Tooltip.Right beyondStarOffset
                            , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                            }
                        , arrow = Just Styles.tooltipArrowSize
                        , containerAttrs = Just Styles.tooltipBody
                        }
                    )

        HoverState.Tooltip (SideBarInstanceGroup _ _ name) _ ->
            Just
                { body = Html.text name
                , attachPosition =
                    { direction = Tooltip.Right beyondStarOffset
                    , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                    }
                , arrow = Just Styles.tooltipArrowSize
                , containerAttrs = Just Styles.tooltipBody
                }

        HoverState.Tooltip SideBarIcon _ ->
            let
                text =
                    if not isSideBarClickable then
                        "no visible pipelines"

                    else if model.sideBarState.isOpen then
                        "hide sidebar"

                    else
                        "show sidebar"
            in
            Just
                { body = Html.text text
                , attachPosition =
                    { direction = Tooltip.Bottom
                    , alignment = Tooltip.Middle <| 2 * Styles.tooltipArrowSize
                    }
                , arrow = Just 5
                , containerAttrs = Just Styles.tooltipBody
                }

        _ ->
            Nothing


allPipelinesSection : Model m -> Maybe (PipelineScoped a) -> List (Html Message)
allPipelinesSection model currentPipeline =
    let
        pipelinesByTeam =
            visiblePipelines model
                |> List.Extra.gatherEqualsBy .teamName
                |> List.map
                    (\( p, ps ) ->
                        ( p.teamName
                        , Concourse.groupPipelinesWithinTeam (p :: ps)
                            |> List.map
                                (\g ->
                                    case g of
                                        Concourse.RegularPipeline p_ ->
                                            RegularPipeline p_

                                        Concourse.InstanceGroup p_ ps_ ->
                                            InstanceGroup p_ ps_
                                )
                        )
                    )
                |> List.filter (Tuple.second >> List.isEmpty >> not)
    in
    [ Html.div Styles.sectionHeader [ Html.text "all pipelines" ]
    , Html.div [ id "all-pipelines" ]
        (pipelinesByTeam
            |> List.map
                (\( teamName, pipelines ) ->
                    Team.team
                        { hovered = model.hovered
                        , pipelines = pipelines
                        , currentPipeline = currentPipeline
                        , favoritedPipelines = model.favoritedPipelines
                        , favoritedInstanceGroups = model.favoritedInstanceGroups
                        , isFavoritesSection = False
                        }
                        { name = teamName
                        , isExpanded = Set.member teamName model.expandedTeamsInAllPipelines
                        }
                        |> Views.viewTeam
                )
        )
    ]


favoritedPipelinesSection : Model m -> Maybe (PipelineScoped a) -> List (Html Message)
favoritedPipelinesSection model currentPipeline =
    let
        extractTeamFavorites pipelines =
            Concourse.groupPipelinesWithinTeam pipelines
                |> List.concatMap
                    (\g ->
                        case g of
                            Concourse.RegularPipeline p ->
                                if Favorites.isPipelineFavorited model p then
                                    [ RegularPipeline p ]

                                else
                                    []

                            Concourse.InstanceGroup p ps ->
                                let
                                    favoritedInstances =
                                        List.filter (Favorites.isPipelineFavorited model) (p :: ps)
                                in
                                (if
                                    Favorites.isInstanceGroupFavorited model (Concourse.toInstanceGroupId p)
                                        || (not << List.isEmpty) favoritedInstances
                                 then
                                    [ InstanceGroup p ps ]

                                 else
                                    []
                                )
                                    ++ (favoritedInstances |> List.map InstancedPipeline)
                    )

        favoritedPipelinesByTeam =
            visiblePipelines model
                |> List.Extra.gatherEqualsBy .teamName
                |> List.map
                    (\( p, ps ) ->
                        ( p.teamName
                        , extractTeamFavorites (p :: ps)
                        )
                    )
                |> List.filter (Tuple.second >> List.isEmpty >> not)
    in
    if List.isEmpty favoritedPipelinesByTeam then
        []

    else
        [ Html.div Styles.sectionHeader [ Html.text "favorite pipelines" ]
        , Html.div [ id "favorites" ]
            (favoritedPipelinesByTeam
                |> List.map
                    (\( teamName, pipelines ) ->
                        Team.team
                            { hovered = model.hovered
                            , pipelines = pipelines
                            , currentPipeline = currentPipeline
                            , favoritedPipelines = model.favoritedPipelines
                            , favoritedInstanceGroups = model.favoritedInstanceGroups
                            , isFavoritesSection = True
                            }
                            { name = teamName
                            , isExpanded =
                                not <|
                                    Set.member teamName model.collapsedTeamsInFavorites
                            }
                            |> Views.viewTeam
                    )
            )
        , Views.Styles.separator 10
        ]


sideBarIcon : Model m -> Html Message
sideBarIcon model =
    if model.screenSize == Mobile then
        Html.text ""

    else
        let
            isSideBarClickable =
                hasVisiblePipelines model

            isOpen =
                model.sideBarState.isOpen

            isHovered =
                HoverState.isHovered SideBarIcon model.hovered

            assetSideBarIcon =
                if not isSideBarClickable then
                    Assets.SideBarIconOpenedGrey

                else if isOpen && isHovered then
                    Assets.SideBarIconClosedWhite

                else if isOpen && not isHovered then
                    Assets.SideBarIconClosedGrey

                else if not isOpen && isHovered then
                    Assets.SideBarIconOpenedWhite

                else
                    Assets.SideBarIconOpenedGrey
        in
        Html.div
            (id "sidebar-icon"
                :: Styles.sideBarMenu isSideBarClickable
                ++ [ onMouseEnter <| Hover <| Just SideBarIcon
                   , onMouseLeave <| Hover Nothing
                   ]
                ++ (if isSideBarClickable then
                        [ onClick <| Click SideBarIcon ]

                    else
                        []
                   )
            )
            [ Icon.icon
                { sizePx = 22, image = assetSideBarIcon }
                []
            ]


visiblePipelines : Model m -> List Concourse.Pipeline
visiblePipelines model =
    model.pipelines
        |> RemoteData.withDefault []
        |> List.filter (isPipelineVisible model)


hasVisiblePipelines : Model m -> Bool
hasVisiblePipelines =
    visiblePipelines >> List.isEmpty >> not


isPipelineVisible : { a | favoritedPipelines : Set Concourse.DatabaseID } -> Concourse.Pipeline -> Bool
isPipelineVisible session p =
    not p.archived || Favorites.isPipelineFavorited session p


updatePipeline :
    (Concourse.Pipeline -> Concourse.Pipeline)
    -> (Concourse.Pipeline -> Bool)
    -> { b | pipelines : WebData (List Concourse.Pipeline) }
    -> { b | pipelines : WebData (List Concourse.Pipeline) }
updatePipeline updater predicate model =
    { model
        | pipelines =
            model.pipelines
                |> RemoteData.map (List.Extra.updateIf predicate updater)
    }


lookupPipeline :
    (Concourse.Pipeline -> Bool)
    -> { b | pipelines : WebData (List Concourse.Pipeline) }
    -> Maybe Concourse.Pipeline
lookupPipeline predicate { pipelines } =
    case pipelines of
        Success ps ->
            List.Extra.find predicate ps

        _ ->
            Nothing


byDatabaseId : Concourse.DatabaseID -> Concourse.Pipeline -> Bool
byDatabaseId id =
    .id >> (==) id


byPipelineId :
    { r | teamName : String, pipelineName : String, pipelineInstanceVars : Concourse.InstanceVars }
    -> Concourse.Pipeline
    -> Bool
byPipelineId pipelineId p =
    (p.name == pipelineId.pipelineName)
        && (p.teamName == pipelineId.teamName)
        && (p.instanceVars == pipelineId.pipelineInstanceVars)


curPipeline : List Concourse.Pipeline -> Routes.Route -> Maybe Concourse.Pipeline
curPipeline pipelines route =
    case route of
        Routes.Build { id } ->
            List.Extra.find (byPipelineId id) pipelines

        Routes.Resource { id } ->
            List.Extra.find (byPipelineId id) pipelines

        Routes.Job { id } ->
            List.Extra.find (byPipelineId id) pipelines

        Routes.Pipeline { id } ->
            List.Extra.find (byPipelineId id) pipelines

        _ ->
            Nothing
