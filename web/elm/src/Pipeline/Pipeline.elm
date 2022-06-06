module Pipeline.Pipeline exposing
    ( Flags
    , Model
    , changeToPipelineAndGroups
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Colors
import Concourse
import Concourse.Cli as Cli
import EffectTransformer exposing (ET)
import Favorites
import HoverState
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , download
        , href
        , id
        , src
        , style
        )
import Html.Attributes.Aria exposing (ariaLabel)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import Http
import Keyboard
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Pipeline.PinMenu.PinMenu as PinMenu
import Pipeline.Styles as Styles
import RemoteData exposing (WebData)
import Routes
import SideBar.SideBar as SideBar
import StrictEvents exposing (onShiftLeftClick)
import Svg
import Svg.Attributes as SvgAttributes
import Time
import Tooltip
import UpdateMsg exposing (UpdateMsg)
import Views.FavoritedIcon as FavoritedIcon
import Views.PauseToggle as PauseToggle
import Views.Styles
import Views.TopBar as TopBar


type alias Model =
    Login.Model
        { pipelineLocator : Concourse.PipelineIdentifier
        , pipeline : WebData Concourse.Pipeline
        , fetchedJobs : Maybe (List Concourse.Job)
        , fetchedResources : Maybe (List Concourse.Resource)
        , renderedJobs : Maybe (List Concourse.Job)
        , renderedResources : Maybe (List Concourse.Resource)
        , turbulenceImgSrc : String
        , experiencingTurbulence : Bool
        , selectedGroups : List String
        , hideLegend : Bool
        , hideLegendCounter : Float
        , isToggleLoading : Bool
        , pinMenuExpanded : Bool
        }


type alias Flags =
    { pipelineLocator : Concourse.PipelineIdentifier
    , turbulenceImgSrc : String
    , selectedGroups : List String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        model =
            { turbulenceImgSrc = flags.turbulenceImgSrc
            , pipelineLocator = flags.pipelineLocator
            , pipeline = RemoteData.NotAsked
            , fetchedJobs = Nothing
            , fetchedResources = Nothing
            , renderedJobs = Nothing
            , renderedResources = Nothing
            , experiencingTurbulence = False
            , hideLegend = False
            , hideLegendCounter = 0
            , isToggleLoading = False
            , selectedGroups = flags.selectedGroups
            , isUserMenuExpanded = False
            , pinMenuExpanded = False
            }
    in
    ( model
    , [ FetchPipeline flags.pipelineLocator
      , ResetPipelineFocus
      , FetchAllPipelines
      , GetCurrentTimeZone
      ]
    )


changeToPipelineAndGroups :
    { pipelineLocator : Concourse.PipelineIdentifier
    , selectedGroups : List String
    }
    -> ET Model
changeToPipelineAndGroups { pipelineLocator, selectedGroups } ( model, effects ) =
    if model.pipelineLocator == pipelineLocator then
        let
            ( newModel, newEffects ) =
                renderIfNeeded ( { model | selectedGroups = selectedGroups }, [] )
        in
        ( newModel, effects ++ newEffects ++ [ ResetPipelineFocus ] )

    else
        let
            ( newModel, newEffects ) =
                init
                    { pipelineLocator = pipelineLocator
                    , selectedGroups = selectedGroups
                    , turbulenceImgSrc = model.turbulenceImgSrc
                    }
        in
        ( newModel, effects ++ newEffects )


timeUntilHidden : Float
timeUntilHidden =
    10 * 1000


timeUntilHiddenCheckInterval : Float
timeUntilHiddenCheckInterval =
    1 * 1000


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.pipeline of
        RemoteData.Failure _ ->
            UpdateMsg.NotFound

        _ ->
            UpdateMsg.AOK


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    let
        redirectToLoginIfUnauthenticated status =
            if status.code == 401 then
                [ RedirectToLogin ]

            else
                []
    in
    case callback of
        PipelineFetched (Ok pipeline) ->
            let
                locator =
                    Concourse.toPipelineId pipeline
            in
            ( { model
                | pipeline = RemoteData.Success pipeline
                , pipelineLocator = locator
              }
            , effects
                ++ [ FetchJobs locator
                   , FetchResources locator
                   ]
            )

        PipelineFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | pipeline = RemoteData.Failure err }
                        , effects
                        )

                    else
                        ( model
                        , effects ++ redirectToLoginIfUnauthenticated status
                        )

                _ ->
                    renderIfNeeded
                        ( { model | experiencingTurbulence = True }
                        , effects
                        )

        PipelineToggled _ (Ok ()) ->
            ( { model
                | pipeline =
                    RemoteData.map
                        (\p -> { p | paused = not p.paused })
                        model.pipeline
                , isToggleLoading = False
              }
            , effects ++ [ FetchPipeline model.pipelineLocator ]
            )

        PipelineToggled _ (Err _) ->
            ( { model | isToggleLoading = False }, effects )

        JobsFetched (Ok fetchedJobs) ->
            renderIfNeeded
                ( { model
                    | fetchedJobs = Just fetchedJobs
                    , experiencingTurbulence = False
                  }
                , effects
                )

        JobsFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded
                        ( { model
                            | fetchedJobs = Nothing
                            , experiencingTurbulence = True
                          }
                        , effects
                        )

        ResourcesFetched (Ok fetchedResources) ->
            renderIfNeeded
                ( { model
                    | fetchedResources = Just fetchedResources
                    , experiencingTurbulence = False
                  }
                , effects
                )

        ResourcesFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded
                        ( { model
                            | fetchedResources = Nothing
                            , experiencingTurbulence = True
                          }
                        , effects
                        )

        ClusterInfoFetched (Ok _) ->
            ( { model
                | experiencingTurbulence = False
              }
            , effects
            )

        ClusterInfoFetched (Err _) ->
            ( { model | experiencingTurbulence = True }, effects )

        AllPipelinesFetched (Err _) ->
            ( { model | experiencingTurbulence = True }, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyEvent ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }
            , if keyEvent.code == Keyboard.F then
                effects ++ [ ResetPipelineFocus ]

              else
                effects
            )

        Moused _ ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }, effects )

        ClockTicked OneSecond _ ->
            if model.hideLegendCounter + timeUntilHiddenCheckInterval > timeUntilHidden then
                ( { model | hideLegend = True }, effects )

            else
                ( { model | hideLegendCounter = model.hideLegendCounter + timeUntilHiddenCheckInterval }
                , effects
                )

        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ FetchPipeline model.pipelineLocator
                   , FetchAllPipelines
                   ]
            )

        ClockTicked OneMinute _ ->
            ( model, effects ++ [ FetchClusterInfo ] )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    (case msg of
        ToggleGroup group ->
            ( model
            , effects
                ++ [ NavigateTo <|
                        getNextUrl
                            (toggleGroup group model.selectedGroups model.pipeline)
                            model
                   ]
            )

        Click (TopBarPauseToggle pipelineIdentifier) ->
            let
                paused =
                    model.pipeline |> RemoteData.map .paused
            in
            case paused of
                RemoteData.Success p ->
                    ( { model | isToggleLoading = True }
                    , effects
                        ++ [ SendTogglePipelineRequest pipelineIdentifier p ]
                    )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )
    )
        |> PinMenu.update msg


subscriptions : List Subscription
subscriptions =
    [ OnClockTick OneMinute
    , OnClockTick FiveSeconds
    , OnClockTick OneSecond
    , OnMouse
    , OnKeyDown
    , OnWindowResize
    ]


documentTitle : Model -> String
documentTitle model =
    model.pipelineLocator.pipelineName


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Pipeline
                { id = model.pipelineLocator
                , groups = model.selectedGroups
                }

        displayPaused =
            isPaused model.pipeline
                && not (isArchived model.pipeline)
    in
    Html.div [ Html.Attributes.style "height" "100%" ]
        [ Html.div
            (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
            [ Html.div
                (id "top-bar-app" :: Views.Styles.topBar displayPaused)
                (SideBar.sideBarIcon session
                    :: TopBar.breadcrumbs session route
                    ++ [ if isArchived model.pipeline then
                            Html.text ""

                         else
                            TopBar.paused
                                { paused = displayPaused
                                , pausedBy = pausedBy model.pipeline
                                , pausedAt = pausedAt model.pipeline
                                , timeZone = session.timeZone
                                }
                       , PinMenu.viewPinMenu session model
                       , Html.div
                            Styles.favoritedIcon
                            [ FavoritedIcon.view
                                { isHovered = HoverState.isHovered (TopBarFavoritedIcon <| getPipelineId model.pipeline) session.hovered
                                , isFavorited =
                                    model.pipeline
                                        |> RemoteData.map (Favorites.isPipelineFavorited session)
                                        |> RemoteData.withDefault False
                                , isSideBar = False
                                , domID = TopBarFavoritedIcon <| getPipelineId model.pipeline
                                }
                                [ style "margin" "17px" ]
                            ]
                       , if isArchived model.pipeline then
                            Html.text ""

                         else
                            Html.div
                                Styles.pauseToggle
                                [ PauseToggle.view
                                    { pipeline = model.pipelineLocator
                                    , isPaused = isPaused model.pipeline
                                    , isToggleHovered =
                                        HoverState.isHovered
                                            (TopBarPauseToggle model.pipelineLocator)
                                            session.hovered
                                    , isToggleLoading = model.isToggleLoading
                                    , tooltipPosition = Views.Styles.Below
                                    , margin = "17px"
                                    , userState = session.userState
                                    , domID = TopBarPauseToggle model.pipelineLocator
                                    }
                                ]
                       , Login.view session.userState model
                       ]
                )
            , Html.div
                (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
              <|
                [ SideBar.view session (Just model.pipelineLocator)
                , viewSubPage session model
                ]
            ]
        ]


tooltip : Model -> Session -> Maybe Tooltip.Tooltip
tooltip model session =
    case session.hovered of
        HoverState.Tooltip (TopBarFavoritedIcon _) _ ->
            let
                isFavorited =
                    RemoteData.map (Favorites.isPipelineFavorited session) model.pipeline
                        |> RemoteData.withDefault False
            in
            Just
                { body =
                    Html.text <|
                        if isFavorited then
                            "unfavorite pipeline"

                        else
                            "favorite pipeline"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (TopBarPauseToggle _) _ ->
            Just
                { body =
                    Html.text <|
                        if isPaused model.pipeline then
                            "unpause pipeline"

                        else
                            "pause pipeline"
                , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
                , arrow = Just 5
                , containerAttrs = Nothing
                }

        HoverState.Tooltip (UserDisplayName username) _ ->
            Login.tooltip username

        _ ->
            PinMenu.tooltip model session


getPipelineId : WebData Concourse.Pipeline -> Int
getPipelineId p =
    RemoteData.withDefault -1 (RemoteData.map .id p)


isPaused : WebData Concourse.Pipeline -> Bool
isPaused p =
    RemoteData.withDefault False (RemoteData.map .paused p)


pausedBy : WebData Concourse.Pipeline -> Maybe String
pausedBy pipeline =
    case pipeline of
        RemoteData.Success p ->
            p.pausedBy

        _ ->
            Nothing


pausedAt : WebData Concourse.Pipeline -> Maybe Time.Posix
pausedAt pipeline =
    case pipeline of
        RemoteData.Success p ->
            p.pausedAt

        _ ->
            Nothing


isArchived : WebData Concourse.Pipeline -> Bool
isArchived p =
    RemoteData.withDefault False (RemoteData.map .archived p)


backgroundImage : WebData Concourse.Pipeline -> List (Html.Attribute msg)
backgroundImage pipeline =
    case pipeline of
        RemoteData.Success p ->
            p.backgroundImage
                |> Maybe.map Styles.pipelineBackground
                |> Maybe.withDefault []

        _ ->
            []


viewSubPage :
    { a | hovered : HoverState.HoverState, version : String }
    -> Model
    -> Html Message
viewSubPage session model =
    Html.div
        [ class "pipeline-view"
        , id "pipeline-container"
        , style "display" "flex"
        , style "flex-direction" "column"
        , style "flex-grow" "1"
        ]
        [ viewGroupsBar session model
        , Html.div
            [ class "pipeline-content" ]
            [ Html.div
                (id "pipeline-background" :: backgroundImage model.pipeline)
                []
            , Svg.svg
                [ SvgAttributes.class "pipeline-graph test" ]
                []
            , Html.div
                [ if model.experiencingTurbulence then
                    class "error-message"

                  else
                    class "error-message hidden"
                ]
                [ Html.div [ class "message" ]
                    [ Html.img [ src model.turbulenceImgSrc, class "seatbelt" ] []
                    , Html.p [] [ Html.text "experiencing turbulence" ]
                    , Html.p [ class "explanation" ] []
                    ]
                ]
            , if model.hideLegend then
                Html.text ""

              else
                Html.dl
                    [ id "legend", class "legend" ]
                    [ Html.dt [ class "succeeded" ] []
                    , Html.dd [] [ Html.text "succeeded" ]
                    , Html.dt [ class "errored" ] []
                    , Html.dd [] [ Html.text "errored" ]
                    , Html.dt [ class "aborted" ] []
                    , Html.dd [] [ Html.text "aborted" ]
                    , Html.dt [ class "paused" ] []
                    , Html.dd [] [ Html.text "paused" ]
                    , Html.dt
                        [ Html.Attributes.style "background-color" Colors.pinned
                        ]
                        []
                    , Html.dd [] [ Html.text "pinned" ]
                    , Html.dt [ class "failed" ] []
                    , Html.dd [] [ Html.text "failed" ]
                    , Html.dt [ class "pending" ] []
                    , Html.dd [] [ Html.text "pending" ]
                    , Html.dt [ class "started" ] []
                    , Html.dd [] [ Html.text "started" ]
                    , Html.dt [ class "dotted" ] [ Html.text "." ]
                    , Html.dd [] [ Html.text "dependency" ]
                    , Html.dt [ class "solid" ] [ Html.text "-" ]
                    , Html.dd [] [ Html.text "dependency (trigger)" ]
                    ]
            , Html.table [ class "lower-right-info" ]
                [ Html.tr []
                    [ Html.td [ class "label" ] [ Html.text "cli:" ]
                    , Html.td []
                        [ Html.ul [ class "cli-downloads" ] <|
                            List.map
                                (\cli ->
                                    Html.li []
                                        [ Html.a
                                            ([ href <| Cli.downloadUrl cli
                                             , ariaLabel <| Cli.label cli
                                             , download ""
                                             ]
                                                ++ Styles.cliIcon cli
                                            )
                                            []
                                        ]
                                )
                                Cli.clis
                        ]
                    ]
                , Html.tr []
                    [ Html.td [ class "label" ] [ Html.text "version:" ]
                    , Html.td []
                        [ Html.div [ id "concourse-version" ]
                            [ Html.text "v"
                            , Html.span
                                [ class "number" ]
                                [ Html.text session.version ]
                            ]
                        ]
                    ]
                ]
            ]
        ]


viewGroupsBar : { a | hovered : HoverState.HoverState } -> Model -> Html Message
viewGroupsBar session model =
    let
        groupList =
            case model.pipeline of
                RemoteData.Success pipeline ->
                    List.indexedMap
                        (viewGroup
                            { selectedGroups = selectedGroupsOrDefault model
                            , pipelineLocator = model.pipelineLocator
                            , hovered = session.hovered
                            }
                        )
                        pipeline.groups

                _ ->
                    []
    in
    if List.isEmpty groupList then
        Html.text ""

    else
        Html.div
            (id "groups-bar" :: Styles.groupsBar)
            groupList


viewGroup :
    { a
        | selectedGroups : List String
        , pipelineLocator : Concourse.PipelineIdentifier
        , hovered : HoverState.HoverState
    }
    -> Int
    -> Concourse.PipelineGroup
    -> Html Message
viewGroup { selectedGroups, pipelineLocator, hovered } idx grp =
    let
        url =
            Routes.toString <|
                Routes.Pipeline { id = pipelineLocator, groups = [ grp.name ] }
    in
    Html.a
        ([ Html.Attributes.href <| url
         , onShiftLeftClick (ToggleGroup grp)
         , onMouseEnter <| Hover <| Just <| JobGroup idx
         , onMouseLeave <| Hover Nothing
         ]
            ++ Styles.groupItem
                { selected = List.member grp.name selectedGroups
                , hovered = HoverState.isHovered (JobGroup idx) hovered
                }
        )
        [ Html.text grp.name ]


jobAppearsInGroups : List String -> Concourse.Job -> Bool
jobAppearsInGroups groupNames job =
    anyIntersect job.groups groupNames


filterJobs : Model -> List Concourse.Job -> List Concourse.Job
filterJobs model jobs =
    List.filter
        (jobAppearsInGroups (activeGroups model))
        jobs


activeGroups : Model -> List String
activeGroups model =
    case ( model.selectedGroups, model.pipeline |> RemoteData.toMaybe |> Maybe.andThen (List.head << .groups) ) of
        ( [], Just firstGroup ) ->
            [ firstGroup.name ]

        ( groups, _ ) ->
            groups


renderIfNeeded : ET Model
renderIfNeeded ( model, effects ) =
    case ( model.fetchedResources, model.fetchedJobs ) of
        ( Just fetchedResources, Just fetchedJobs ) ->
            let
                filteredFetchedJobs =
                    if List.isEmpty (activeGroups model) then
                        fetchedJobs

                    else
                        filterJobs model fetchedJobs
            in
            case ( model.renderedResources, model.renderedJobs ) of
                ( Just renderedResources, Just renderedJobs ) ->
                    if
                        (renderedJobs /= filteredFetchedJobs)
                            || (renderedResources /= fetchedResources)
                    then
                        ( { model
                            | renderedJobs = Just filteredFetchedJobs
                            , renderedResources = Just fetchedResources
                          }
                        , effects ++ [ RenderPipeline filteredFetchedJobs fetchedResources ]
                        )

                    else
                        ( model, effects )

                _ ->
                    ( { model
                        | renderedJobs = Just filteredFetchedJobs
                        , renderedResources = Just fetchedResources
                      }
                    , effects ++ [ RenderPipeline filteredFetchedJobs fetchedResources ]
                    )

        _ ->
            ( model, effects )


anyIntersect : List a -> List a -> Bool
anyIntersect list1 list2 =
    case list1 of
        [] ->
            False

        first :: rest ->
            if List.member first list2 then
                True

            else
                anyIntersect rest list2


toggleGroup : Concourse.PipelineGroup -> List String -> WebData Concourse.Pipeline -> List String
toggleGroup grp names mpipeline =
    if List.member grp.name names then
        List.filter ((/=) grp.name) names

    else if List.isEmpty names then
        grp.name :: getDefaultSelectedGroups mpipeline

    else
        grp.name :: names


selectedGroupsOrDefault : Model -> List String
selectedGroupsOrDefault model =
    if List.isEmpty model.selectedGroups then
        getDefaultSelectedGroups model.pipeline

    else
        model.selectedGroups


getDefaultSelectedGroups : WebData Concourse.Pipeline -> List String
getDefaultSelectedGroups pipeline =
    case pipeline of
        RemoteData.Success p ->
            case List.head p.groups of
                Nothing ->
                    []

                Just first ->
                    [ first.name ]

        _ ->
            []


getNextUrl : List String -> Model -> String
getNextUrl newGroups model =
    Routes.toString <|
        Routes.Pipeline { id = model.pipelineLocator, groups = newGroups }
