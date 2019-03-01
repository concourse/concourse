module Resource.Resource exposing
    ( Flags
    , changeToResource
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    , viewPinButton
    , viewVersionBody
    , viewVersionHeader
    )

import Callback exposing (Callback(..))
import Colors
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , Pagination
        , chevron
        , chevronContainer
        , equal
        )
import Css
import Date exposing (Date)
import Date.Format
import Dict
import DictView
import Duration exposing (Duration)
import Effects exposing (Effect(..), runEffect, setTitle)
import Html as UnstyledHtml
import Html.Attributes
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes
    exposing
        ( attribute
        , class
        , css
        , href
        , id
        , placeholder
        , style
        , title
        , value
        )
import Html.Styled.Events
    exposing
        ( onBlur
        , onClick
        , onFocus
        , onInput
        , onMouseEnter
        , onMouseLeave
        , onMouseOut
        , onMouseOver
        )
import Http
import Keycodes
import List.Extra
import Maybe.Extra as ME
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import Resource.Models as Models exposing (Model)
import Resource.Msgs exposing (Msg(..))
import Resource.Styles
import Routes
import Spinner
import StrictEvents
import Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import Time exposing (Time)
import TopBar.Model
import TopBar.Styles
import TopBar.TopBar as TopBar
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState(..))


type alias Flags =
    { resourceId : Concourse.ResourceIdentifier
    , paging : Maybe Concourse.Pagination.Page
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            TopBar.init { route = Routes.Resource { id = flags.resourceId, page = Nothing } }

        model =
            { resourceIdentifier = flags.resourceId
            , pageStatus = Err Models.Empty
            , checkStatus = Models.CheckingSuccessfully
            , checkError = ""
            , checkSetupError = ""
            , hovered = Models.None
            , lastChecked = Nothing
            , pinnedVersion = NotPinned
            , currentPage = flags.paging
            , versions =
                { content = []
                , pagination = { previousPage = Nothing, nextPage = Nothing }
                }
            , now = Nothing
            , showPinBarTooltip = False
            , pinIconHover = False
            , pinCommentLoading = False
            , ctrlDown = False
            , textAreaFocused = False
            , isUserMenuExpanded = topBar.isUserMenuExpanded
            , isPinMenuExpanded = topBar.isPinMenuExpanded
            , route = topBar.route
            , groups = topBar.groups
            , query = topBar.query
            , dropdown = topBar.dropdown
            , screenSize = topBar.screenSize
            , shiftDown = topBar.shiftDown
            }
    in
    ( model
    , topBarEffects ++ [ FetchResource flags.resourceId, FetchVersionedResources flags.resourceId flags.paging ]
    )


changeToResource : Flags -> Model -> ( Model, List Effect )
changeToResource flags model =
    ( { model
        | currentPage = flags.paging
        , versions =
            { content = []
            , pagination = { previousPage = Nothing, nextPage = Nothing }
            }
      }
    , [ FetchVersionedResources model.resourceIdentifier flags.paging ]
    )


updatePinnedVersion : Concourse.Resource -> Model -> Model
updatePinnedVersion resource model =
    case ( resource.pinnedVersion, resource.pinnedInConfig ) of
        ( Nothing, _ ) ->
            case model.pinnedVersion of
                PinningTo _ ->
                    model

                _ ->
                    { model | pinnedVersion = NotPinned }

        ( Just v, True ) ->
            { model | pinnedVersion = PinnedStaticallyTo v }

        ( Just newVersion, False ) ->
            let
                pristineComment =
                    resource.pinComment |> Maybe.withDefault ""
            in
            case model.pinnedVersion of
                UnpinningFrom c _ ->
                    { model | pinnedVersion = UnpinningFrom c newVersion }

                PinnedDynamicallyTo { comment } _ ->
                    { model
                        | pinnedVersion =
                            PinnedDynamicallyTo
                                { comment = comment
                                , pristineComment = pristineComment
                                }
                                newVersion
                    }

                _ ->
                    { model
                        | pinnedVersion =
                            PinnedDynamicallyTo
                                { comment = pristineComment
                                , pristineComment = pristineComment
                                }
                                newVersion
                    }


hasPinnedVersion : Model -> Concourse.Version -> Bool
hasPinnedVersion model v =
    case model.pinnedVersion of
        PinnedStaticallyTo pv ->
            v == pv

        PinnedDynamicallyTo _ pv ->
            v == pv

        UnpinningFrom _ pv ->
            v == pv

        _ ->
            False


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    if model.pageStatus == Err Models.NotFound then
        UpdateMsg.NotFound

    else
        UpdateMsg.AOK


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnClockTick Subscription.FiveSeconds
    , OnClockTick Subscription.OneSecond
    , OnKeyDown
    , OnKeyUp
    ]


handleCallback : Callback -> ( Model, List Effect ) -> ( Model, List Effect )
handleCallback msg =
    TopBar.handleCallback msg >> handleCallbackWithoutTopBar msg


handleCallbackWithoutTopBar : Callback -> ( Model, List Effect ) -> ( Model, List Effect )
handleCallbackWithoutTopBar action ( model, effects ) =
    case action of
        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , resourceIdentifier =
                    { teamName = resource.teamName
                    , pipelineName = resource.pipelineName
                    , resourceName = resource.name
                    }
                , checkStatus =
                    if resource.failingToCheck then
                        Models.FailingToCheck

                    else
                        Models.CheckingSuccessfully
                , checkError = resource.checkError
                , checkSetupError = resource.checkSetupError
                , lastChecked = resource.lastChecked
              }
                |> updatePinnedVersion resource
            , effects ++ [ SetTitle <| resource.name ++ " - " ]
            )

        ResourceFetched (Err err) ->
            case Debug.log "failed to fetch resource" err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err Models.NotFound }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        VersionedResourcesFetched (Ok ( requestedPage, paginated )) ->
            let
                fetchedPage =
                    permalink paginated.content

                versions =
                    { pagination = paginated.pagination
                    , content =
                        paginated.content
                            |> List.map
                                (\vr ->
                                    let
                                        existingVersion : Maybe Models.Version
                                        existingVersion =
                                            model.versions.content
                                                |> List.Extra.find
                                                    (\v ->
                                                        v.id.versionID == vr.id
                                                    )

                                        enabledStateAccordingToServer : Models.VersionEnabledState
                                        enabledStateAccordingToServer =
                                            if vr.enabled then
                                                Models.Enabled

                                            else
                                                Models.Disabled
                                    in
                                    case existingVersion of
                                        Just ev ->
                                            { ev
                                                | enabled =
                                                    if ev.enabled == Models.Changing then
                                                        Models.Changing

                                                    else
                                                        enabledStateAccordingToServer
                                            }

                                        Nothing ->
                                            { id =
                                                { teamName = model.resourceIdentifier.teamName
                                                , pipelineName = model.resourceIdentifier.pipelineName
                                                , resourceName = model.resourceIdentifier.resourceName
                                                , versionID = vr.id
                                                }
                                            , version = vr.version
                                            , metadata = vr.metadata
                                            , enabled = enabledStateAccordingToServer
                                            , expanded = False
                                            , inputTo = []
                                            , outputOf = []
                                            , showTooltip = False
                                            }
                                )
                    }

                newModel =
                    \newPage ->
                        { model
                            | versions = versions
                            , currentPage = newPage
                        }

                chosenModelWith =
                    \requestedPageUnwrapped ->
                        case model.currentPage of
                            Nothing ->
                                newModel <| Just fetchedPage

                            Just page ->
                                if Concourse.Pagination.equal page requestedPageUnwrapped then
                                    newModel <| requestedPage

                                else
                                    model
            in
            case requestedPage of
                Nothing ->
                    ( newModel (Just fetchedPage), effects )

                Just requestedPageUnwrapped ->
                    ( chosenModelWith requestedPageUnwrapped
                    , effects
                    )

        VersionedResourcesFetched (Err err) ->
            flip always (Debug.log "failed to fetch versioned resources" err) <|
                ( model, effects )

        InputToFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | inputTo = builds }) model
            , effects
            )

        OutputOfFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | outputOf = builds }) model
            , effects
            )

        VersionPinned (Ok ()) ->
            let
                newPinnedVersion =
                    Pinned.finishPinning
                        (\pinningTo ->
                            model.versions.content
                                |> List.Extra.find (\v -> v.id == pinningTo)
                                |> Maybe.map .version
                        )
                        model.pinnedVersion
            in
            ( { model | pinnedVersion = newPinnedVersion }, effects )

        VersionPinned (Err _) ->
            ( { model | pinnedVersion = NotPinned }
            , effects
            )

        VersionUnpinned (Ok ()) ->
            ( { model | pinnedVersion = NotPinned }
            , effects ++ [ FetchResource model.resourceIdentifier ]
            )

        VersionUnpinned (Err _) ->
            ( { model | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion }
            , effects
            )

        VersionToggled action versionID result ->
            let
                newEnabledState : Models.VersionEnabledState
                newEnabledState =
                    case ( result, action ) of
                        ( Ok (), Models.Enable ) ->
                            Models.Enabled

                        ( Ok (), Models.Disable ) ->
                            Models.Disabled

                        ( Err _, Models.Enable ) ->
                            Models.Disabled

                        ( Err _, Models.Disable ) ->
                            Models.Enabled
            in
            ( updateVersion versionID (\v -> { v | enabled = newEnabledState }) model
            , effects
            )

        Checked (Ok ()) ->
            ( { model | checkStatus = Models.CheckingSuccessfully }
            , effects
                ++ [ FetchResource model.resourceIdentifier
                   , FetchVersionedResources
                        model.resourceIdentifier
                        model.currentPage
                   ]
            )

        Checked (Err err) ->
            ( { model | checkStatus = Models.FailingToCheck }
            , case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        effects ++ [ RedirectToLogin ]

                    else
                        effects ++ [ FetchResource model.resourceIdentifier ]

                _ ->
                    effects
            )

        CommentSet result ->
            ( { model
                | pinCommentLoading = False
                , pinnedVersion =
                    case ( result, model.pinnedVersion ) of
                        ( Ok (), PinnedDynamicallyTo { comment } v ) ->
                            PinnedDynamicallyTo
                                { comment = comment
                                , pristineComment = comment
                                }
                                v

                        ( _, pv ) ->
                            pv
              }
            , effects ++ [ FetchResource model.resourceIdentifier ]
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ( Model, List Effect ) -> ( Model, List Effect )
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keycode ->
            if Keycodes.isControlModifier keycode then
                ( { model | ctrlDown = True }, effects )

            else if keycode == Keycodes.enter && model.ctrlDown && model.textAreaFocused then
                ( model
                , case model.pinnedVersion of
                    PinnedDynamicallyTo { comment } _ ->
                        effects ++ [ SetPinComment model.resourceIdentifier comment ]

                    _ ->
                        effects
                )

            else
                ( model, effects )

        KeyUp keycode ->
            if Keycodes.isControlModifier keycode then
                ( { model | ctrlDown = False }, effects )

            else
                ( model, effects )

        ClockTicked OneSecond time ->
            ( { model | now = Just time }, effects )

        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ FetchResource model.resourceIdentifier
                   , FetchVersionedResources model.resourceIdentifier model.currentPage
                   ]
                ++ fetchDataForExpandedVersions model
            )

        _ ->
            ( model, effects )


update : Msg -> ( Model, List Effect ) -> ( Model, List Effect )
update action ( model, effects ) =
    case action of
        LoadPage page ->
            ( { model
                | currentPage = Just page
              }
            , effects
                ++ [ FetchVersionedResources model.resourceIdentifier <| Just page
                   , NavigateTo <|
                        Routes.toString <|
                            Routes.Resource { id = model.resourceIdentifier, page = Just page }
                   ]
            )

        ExpandVersionedResource versionID ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (.id >> (==) versionID)

                newExpandedState : Bool
                newExpandedState =
                    case version of
                        Just v ->
                            not v.expanded

                        Nothing ->
                            False
            in
            ( updateVersion
                versionID
                (\v ->
                    { v | expanded = newExpandedState }
                )
                model
            , if newExpandedState then
                effects
                    ++ [ FetchInputTo versionID
                       , FetchOutputOf versionID
                       ]

              else
                effects
            )

        NavTo route ->
            ( model, effects ++ [ NavigateTo <| Routes.toString route ] )

        TogglePinBarTooltip ->
            ( { model
                | showPinBarTooltip =
                    case model.pinnedVersion of
                        PinnedStaticallyTo _ ->
                            not model.showPinBarTooltip

                        _ ->
                            False
              }
            , effects
            )

        ToggleVersionTooltip ->
            let
                pinnedVersionID : Maybe Models.VersionId
                pinnedVersionID =
                    model.versions.content
                        |> List.Extra.find (.version >> hasPinnedVersion model)
                        |> Maybe.map .id

                newModel =
                    case ( model.pinnedVersion, pinnedVersionID ) of
                        ( PinnedStaticallyTo _, Just id ) ->
                            updateVersion
                                id
                                (\v -> { v | showTooltip = not v.showTooltip })
                                model

                        _ ->
                            model
            in
            ( newModel, effects )

        PinVersion versionID ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (\v -> v.id == versionID)
            in
            ( { model
                | pinnedVersion =
                    Pinned.startPinningTo
                        versionID
                        model.pinnedVersion
              }
            , case version of
                Just v ->
                    effects ++ [ DoPinVersion versionID ]

                Nothing ->
                    effects
            )

        UnpinVersion ->
            ( { model | pinnedVersion = Pinned.startUnpinning model.pinnedVersion }
            , effects ++ [ DoUnpinVersion model.resourceIdentifier ]
            )

        ToggleVersion action versionID ->
            ( updateVersion versionID
                (\v ->
                    { v | enabled = Models.Changing }
                )
                model
            , effects ++ [ DoToggleVersion action versionID ]
            )

        PinIconHover state ->
            ( { model | pinIconHover = state }, effects )

        Hover hovered ->
            ( { model | hovered = hovered }, effects )

        CheckRequested isAuthorized ->
            if isAuthorized then
                ( { model | checkStatus = Models.CurrentlyChecking }
                , effects ++ [ DoCheck model.resourceIdentifier ]
                )

            else
                ( model, effects ++ [ RedirectToLogin ] )

        TopBarMsg msg ->
            TopBar.update msg ( model, effects )

        EditComment input ->
            let
                newPinnedVersion =
                    case model.pinnedVersion of
                        PinnedDynamicallyTo { pristineComment } v ->
                            PinnedDynamicallyTo
                                { comment = input
                                , pristineComment = pristineComment
                                }
                                v

                        x ->
                            x
            in
            ( { model | pinnedVersion = newPinnedVersion }, effects )

        SaveComment comment ->
            ( { model | pinCommentLoading = True }
            , effects ++ [ SetPinComment model.resourceIdentifier comment ]
            )

        FocusTextArea ->
            ( { model | textAreaFocused = True }, effects )

        BlurTextArea ->
            ( { model | textAreaFocused = False }, effects )


updateVersion :
    Models.VersionId
    -> (Models.Version -> Models.Version)
    -> Model
    -> Model
updateVersion versionID updateFunc model =
    let
        newVersionsContent : List Models.Version
        newVersionsContent =
            model.versions.content
                |> List.Extra.updateIf (.id >> (==) versionID) updateFunc

        versions : Paginated Models.Version
        versions =
            model.versions
    in
    { model | versions = { versions | content = newVersionsContent } }


permalink : List Concourse.VersionedResource -> Page
permalink versionedResources =
    case List.head versionedResources of
        Nothing ->
            { direction = Concourse.Pagination.Since 0
            , limit = 100
            }

        Just version ->
            { direction = Concourse.Pagination.From version.id
            , limit = List.length versionedResources
            }


view : UserState -> Model -> Html Msg
view userState model =
    Html.div []
        [ Html.div
            [ style TopBar.Styles.pageIncludingTopBar, id "page-including-top-bar" ]
            [ Html.map TopBarMsg <| TopBar.view userState TopBar.Model.None model
            , Html.div [ id "page-below-top-bar", style TopBar.Styles.pageBelowTopBar ]
                [ subpageView userState model
                , commentBar userState model
                ]
            ]
        ]


subpageView : UserState -> Model -> Html Msg
subpageView userState model =
    if model.pageStatus == Err Models.Empty then
        Html.div [] []

    else
        Html.div []
            [ header model
            , body userState model
            ]


header : Model -> Html Msg
header model =
    let
        lastCheckedView =
            case ( model.now, model.lastChecked ) of
                ( Just now, Just date ) ->
                    viewLastChecked now date

                ( _, _ ) ->
                    Html.text ""

        headerHeight =
            60
    in
    Html.div
        [ css
            [ Css.height <| Css.px headerHeight
            , Css.position Css.fixed
            , Css.top <| Css.px TopBar.Styles.pageHeaderHeight
            , Css.displayFlex
            , Css.alignItems Css.stretch
            , Css.width <| Css.pct 100
            , Css.zIndex <| Css.int 1
            , Css.backgroundColor <| Css.hex "2a2929"
            ]
        ]
        [ Html.h1
            [ css
                [ Css.fontWeight <| Css.int 700
                , Css.marginLeft <| Css.px 18
                , Css.displayFlex
                , Css.alignItems Css.center
                , Css.justifyContent Css.center
                ]
            ]
            [ Html.text model.resourceIdentifier.resourceName ]
        , Html.div
            [ css
                [ Css.displayFlex
                , Css.alignItems Css.center
                , Css.justifyContent Css.center
                , Css.marginLeft (Css.px 24)
                ]
            ]
            [ lastCheckedView ]
        , pinBar model
        , paginationMenu model
        ]


body : UserState -> Model -> Html Msg
body userState model =
    let
        headerHeight =
            60

        sectionModel =
            { checkStatus = model.checkStatus
            , checkSetupError = model.checkSetupError
            , checkError = model.checkError
            , hovered = model.hovered
            , userState = userState
            , teamName = model.resourceIdentifier.teamName
            }
    in
    Html.div
        [ css
            [ Css.padding3
                (Css.px <| headerHeight + 10)
                (Css.px 10)
                (Css.px 10)
            ]
        , id "body"
        , style
            [ ( "padding-bottom"
              , case model.pinnedVersion of
                    PinnedDynamicallyTo _ _ ->
                        "300px"

                    _ ->
                        ""
              )
            ]
        ]
        [ checkSection sectionModel
        , viewVersionedResources model
        ]


paginationMenu :
    { a
        | versions : Paginated Models.Version
        , resourceIdentifier : Concourse.ResourceIdentifier
        , hovered : Models.Hoverable
    }
    -> Html Msg
paginationMenu { versions, resourceIdentifier, hovered } =
    let
        previousButtonEventHandler =
            case versions.pagination.previousPage of
                Nothing ->
                    []

                Just pp ->
                    [ onClick <| LoadPage pp ]

        nextButtonEventHandler =
            case versions.pagination.nextPage of
                Nothing ->
                    []

                Just np ->
                    let
                        updatedPage =
                            { np | limit = 100 }
                    in
                    [ onClick <| LoadPage updatedPage ]
    in
    Html.div
        [ id "pagination"
        , style
            [ ( "display", "flex" )
            , ( "align-items", "stretch" )
            ]
        ]
        [ case versions.pagination.previousPage of
            Nothing ->
                Html.div
                    [ style chevronContainer ]
                    [ Html.div
                        [ style <|
                            chevron
                                { direction = "left"
                                , enabled = False
                                , hovered = False
                                }
                        ]
                        []
                    ]

            Just page ->
                Html.div
                    ([ style chevronContainer
                     , onMouseEnter <| Hover Models.PreviousPage
                     , onMouseLeave <| Hover Models.None
                     ]
                        ++ previousButtonEventHandler
                    )
                    [ Html.a
                        [ href <|
                            Routes.toString <|
                                Routes.Resource { id = resourceIdentifier, page = Just page }
                        , attribute "aria-label" "Previous Page"
                        , style <|
                            chevron
                                { direction = "left"
                                , enabled = True
                                , hovered = hovered == Models.PreviousPage
                                }
                        ]
                        []
                    ]
        , case versions.pagination.nextPage of
            Nothing ->
                Html.div
                    [ style chevronContainer ]
                    [ Html.div
                        [ style <|
                            chevron
                                { direction = "right"
                                , enabled = False
                                , hovered = False
                                }
                        ]
                        []
                    ]

            Just page ->
                Html.div
                    ([ style chevronContainer
                     , onMouseEnter <| Hover Models.NextPage
                     , onMouseLeave <| Hover Models.None
                     ]
                        ++ nextButtonEventHandler
                    )
                    [ Html.a
                        [ href <|
                            Routes.toString <|
                                Routes.Resource { id = resourceIdentifier, page = Just page }
                        , attribute "aria-label" "Next Page"
                        , style <|
                            chevron
                                { direction = "right"
                                , enabled = True
                                , hovered = hovered == Models.NextPage
                                }
                        ]
                        []
                    ]
        ]


checkSection :
    { a
        | checkStatus : Models.CheckStatus
        , checkSetupError : String
        , checkError : String
        , hovered : Models.Hoverable
        , userState : UserState
        , teamName : String
    }
    -> Html Msg
checkSection ({ checkStatus, checkSetupError, checkError } as model) =
    let
        failingToCheck =
            checkStatus == Models.FailingToCheck

        checkMessage =
            case checkStatus of
                Models.FailingToCheck ->
                    "checking failed"

                Models.CurrentlyChecking ->
                    "currently checking"

                Models.CheckingSuccessfully ->
                    "checking successfully"

        stepBody =
            if failingToCheck then
                if not (String.isEmpty checkSetupError) then
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkSetupError ]
                        ]
                    ]

                else
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkError ]
                        ]
                    ]

            else
                []

        statusIcon =
            case checkStatus of
                Models.CurrentlyChecking ->
                    Html.fromUnstyled <|
                        Spinner.spinner "14px"
                            [ Html.Attributes.style
                                [ ( "margin", "7px" )
                                ]
                            ]

                _ ->
                    Html.div
                        [ style <|
                            Resource.Styles.checkStatusIcon failingToCheck
                        ]
                        []

        statusBar =
            Html.div
                [ style
                    [ ( "display", "flex" )
                    , ( "justify-content", "space-between" )
                    , ( "align-items", "center" )
                    , ( "flex-grow", "1" )
                    , ( "height", "28px" )
                    , ( "background", Colors.sectionHeader )
                    , ( "padding-left", "5px" )
                    ]
                ]
                [ Html.h3 [] [ Html.text checkMessage ]
                , statusIcon
                ]

        checkBar =
            Html.div
                [ style [ ( "display", "flex" ) ] ]
                [ checkButton model, statusBar ]
    in
    Html.div [ class "resource-check-status" ] <| checkBar :: stepBody


checkButton :
    { a
        | hovered : Models.Hoverable
        , userState : UserState
        , teamName : String
        , checkStatus : Models.CheckStatus
    }
    -> Html Msg
checkButton ({ hovered, userState, teamName, checkStatus } as params) =
    let
        isHovered =
            hovered == Models.CheckButton

        isCurrentlyChecking =
            checkStatus == Models.CurrentlyChecking

        isUnauthenticated =
            case userState of
                UserStateLoggedIn _ ->
                    False

                _ ->
                    True

        isUserAuthorized =
            isAuthorized params

        isClickable =
            (isUnauthenticated || isUserAuthorized)
                && not isCurrentlyChecking

        isHighlighted =
            (isClickable && isHovered) || isCurrentlyChecking
    in
    Html.div
        ([ style
            [ ( "height", "28px" )
            , ( "width", "28px" )
            , ( "background-color", Colors.sectionHeader )
            , ( "margin-right", "5px" )
            , ( "cursor"
              , if isClickable then
                    "pointer"

                else
                    "default"
              )
            ]
         , onMouseEnter <| Hover Models.CheckButton
         , onMouseLeave <| Hover Models.None
         ]
            ++ (if isClickable then
                    [ onClick (CheckRequested isUserAuthorized) ]

                else
                    []
               )
        )
        [ Html.div
            [ style
                [ ( "height", "20px" )
                , ( "width", "20px" )
                , ( "margin", "4px" )
                , ( "background-image"
                  , "url(/public/images/baseline-refresh-24px.svg)"
                  )
                , ( "background-position", "50% 50%" )
                , ( "background-repeat", "no-repeat" )
                , ( "background-size", "contain" )
                , ( "opacity"
                  , if isHighlighted then
                        "1"

                    else
                        "0.5"
                  )
                ]
            ]
            []
        ]


isAuthorized : { a | teamName : String, userState : UserState } -> Bool
isAuthorized { teamName, userState } =
    case userState of
        UserStateLoggedIn user ->
            case Dict.get teamName user.teams of
                Just roles ->
                    List.member "member" roles
                        || List.member "owner" roles

                Nothing ->
                    False

        _ ->
            False


commentBar :
    UserState
    ->
        { a
            | pinnedVersion : Models.PinnedVersion
            , resourceIdentifier : Concourse.ResourceIdentifier
            , hovered : Models.Hoverable
            , pinCommentLoading : Bool
        }
    -> Html Msg
commentBar userState ({ resourceIdentifier, pinnedVersion, hovered, pinCommentLoading } as params) =
    case pinnedVersion of
        PinnedDynamicallyTo commentState v ->
            let
                version =
                    viewVersion
                        [ Html.Attributes.style [ ( "align-self", "center" ) ] ]
                        v
            in
            Html.div
                [ id "comment-bar", style Resource.Styles.commentBar ]
                [ Html.div
                    [ style Resource.Styles.commentBarContent ]
                  <|
                    let
                        header =
                            Html.div
                                [ style Resource.Styles.commentBarHeader ]
                                [ Html.div
                                    [ style
                                        Resource.Styles.commentBarIconContainer
                                    ]
                                    [ Html.div
                                        [ style
                                            Resource.Styles.commentBarMessageIcon
                                        ]
                                        []
                                    , Html.div
                                        [ style
                                            Resource.Styles.commentBarPinIcon
                                        ]
                                        []
                                    ]
                                , version
                                ]
                    in
                    if isAuthorized { teamName = resourceIdentifier.teamName, userState = userState } then
                        [ header
                        , Html.textarea
                            [ style Resource.Styles.commentTextArea
                            , onInput EditComment
                            , value commentState.comment
                            , placeholder "enter a comment"
                            , onFocus FocusTextArea
                            , onBlur BlurTextArea
                            ]
                            []
                        , Html.button
                            [ style <|
                                let
                                    commentChanged =
                                        commentState.comment
                                            /= commentState.pristineComment
                                in
                                Resource.Styles.commentSaveButton
                                    { isHovered =
                                        not pinCommentLoading
                                            && commentChanged
                                            && hovered
                                            == Models.SaveComment
                                    , commentChanged = commentChanged
                                    }
                            , onMouseEnter <| Hover Models.SaveComment
                            , onMouseLeave <| Hover Models.None
                            , onClick <| SaveComment commentState.comment
                            ]
                            (if pinCommentLoading then
                                [ Spinner.spinner "12px" []
                                    |> Html.fromUnstyled
                                ]

                             else
                                [ Html.text "save" ]
                            )
                        ]

                    else
                        [ header
                        , Html.pre
                            [ style Resource.Styles.commentText ]
                            [ Html.text commentState.pristineComment ]
                        , Html.div [ style [ ( "height", "24px" ) ] ] []
                        ]
                ]

        _ ->
            Html.text ""


pinBar :
    { a
        | pinnedVersion : Models.PinnedVersion
        , showPinBarTooltip : Bool
        , pinIconHover : Bool
    }
    -> Html Msg
pinBar { pinnedVersion, showPinBarTooltip, pinIconHover } =
    let
        pinBarVersion =
            Pinned.stable pinnedVersion

        attrList : List ( Html.Attribute Msg, Bool ) -> List (Html.Attribute Msg)
        attrList =
            List.filter Tuple.second >> List.map Tuple.first

        isPinnedStatically =
            case pinnedVersion of
                PinnedStaticallyTo _ ->
                    True

                _ ->
                    False

        isPinnedDynamically =
            case pinnedVersion of
                PinnedDynamicallyTo _ _ ->
                    True

                _ ->
                    False
    in
    Html.div
        (attrList
            [ ( id "pin-bar", True )
            , ( style <| Resource.Styles.pinBar { isPinned = ME.isJust pinBarVersion }, True )
            , ( onMouseEnter TogglePinBarTooltip, isPinnedStatically )
            , ( onMouseLeave TogglePinBarTooltip, isPinnedStatically )
            ]
        )
        ([ Html.div
            (attrList
                [ ( id "pin-icon", True )
                , ( style <|
                        Resource.Styles.pinIcon
                            { isPinned = ME.isJust pinBarVersion
                            , isPinnedDynamically = isPinnedDynamically
                            , hover = pinIconHover
                            }
                  , True
                  )
                , ( onClick UnpinVersion, isPinnedDynamically )
                , ( onMouseEnter <| PinIconHover True, isPinnedDynamically )
                , ( onMouseLeave <| PinIconHover False, True )
                ]
            )
            []
         ]
            ++ (case pinBarVersion of
                    Just v ->
                        [ viewVersion [] v ]

                    _ ->
                        []
               )
            ++ (if showPinBarTooltip then
                    [ Html.div
                        [ id "pin-bar-tooltip"
                        , style Resource.Styles.pinBarTooltip
                        ]
                        [ Html.text "pinned in pipeline config" ]
                    ]

                else
                    []
               )
        )


viewVersionedResources :
    { a
        | versions : Paginated Models.Version
        , pinnedVersion : Models.PinnedVersion
    }
    -> Html Msg
viewVersionedResources { versions, pinnedVersion } =
    versions.content
        |> List.map
            (\v ->
                viewVersionedResource
                    { version = v
                    , pinnedVersion = pinnedVersion
                    }
            )
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { version : Models.Version
    , pinnedVersion : Models.PinnedVersion
    }
    -> Html Msg
viewVersionedResource { version, pinnedVersion } =
    let
        pinState =
            case Pinned.pinState version.version version.id pinnedVersion of
                PinnedStatically _ ->
                    PinnedStatically { showTooltip = version.showTooltip }

                x ->
                    x
    in
    Html.li
        (case ( pinState, version.enabled ) of
            ( Disabled, _ ) ->
                [ style [ ( "opacity", "0.5" ) ] ]

            ( _, Models.Disabled ) ->
                [ style [ ( "opacity", "0.5" ) ] ]

            _ ->
                []
        )
        ([ Html.div
            [ css
                [ Css.displayFlex
                , Css.margin2 (Css.px 5) Css.zero
                ]
            ]
            [ viewEnabledCheckbox
                { enabled = version.enabled
                , id = version.id
                , pinState = pinState
                }
            , viewPinButton
                { versionID = version.id
                , pinState = pinState
                , showTooltip = version.showTooltip
                }
            , viewVersionHeader
                { id = version.id
                , version = version.version
                , pinnedState = pinState
                }
            ]
         ]
            ++ (if version.expanded then
                    [ viewVersionBody
                        { inputTo = version.inputTo
                        , outputOf = version.outputOf
                        , metadata = version.metadata
                        }
                    ]

                else
                    []
               )
        )


viewVersionBody :
    { a
        | inputTo : List Concourse.Build
        , outputOf : List Concourse.Build
        , metadata : Concourse.Metadata
    }
    -> Html Msg
viewVersionBody { inputTo, outputOf, metadata } =
    Html.div
        [ css
            [ Css.displayFlex
            , Css.padding2 (Css.px 5) (Css.px 10)
            ]
        ]
        [ Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "inputs to" ] ]
                , viewBuilds <| listToMap inputTo
                ]
        , Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "outputs of" ] ]
                , viewBuilds <| listToMap outputOf
                ]
        , Html.div [ class "vri metadata-container" ]
            [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
            , viewMetadata metadata
            ]
        ]


viewEnabledCheckbox :
    { a
        | enabled : Models.VersionEnabledState
        , id : Models.VersionId
        , pinState : VersionPinState
    }
    -> Html Msg
viewEnabledCheckbox ({ enabled, id, pinState } as params) =
    let
        clickHandler =
            case enabled of
                Models.Enabled ->
                    [ onClick <| ToggleVersion Models.Disable id ]

                Models.Changing ->
                    []

                Models.Disabled ->
                    [ onClick <| ToggleVersion Models.Enable id ]
    in
    Html.div
        ([ Html.Styled.Attributes.attribute
            "aria-label"
            "Toggle Resource Version Enabled"
         , style <| Resource.Styles.enabledCheckbox params
         ]
            ++ clickHandler
        )
        (case enabled of
            Models.Enabled ->
                []

            Models.Changing ->
                [ Html.fromUnstyled <|
                    Spinner.spinner
                        "12.5px"
                        [ Html.Attributes.style [ ( "margin", "6.25px" ) ] ]
                ]

            Models.Disabled ->
                []
        )


viewPinButton :
    { versionID : Models.VersionId
    , pinState : VersionPinState
    , showTooltip : Bool
    }
    -> Html Msg
viewPinButton { versionID, pinState } =
    let
        eventHandlers =
            case pinState of
                Enabled ->
                    [ onClick <| PinVersion versionID ]

                PinnedDynamically ->
                    [ onClick UnpinVersion ]

                PinnedStatically _ ->
                    [ onMouseOut ToggleVersionTooltip
                    , onMouseOver ToggleVersionTooltip
                    ]

                Disabled ->
                    []

                InTransition ->
                    []
    in
    Html.div
        ([ Html.Styled.Attributes.attribute
            "aria-label"
            "Pin Resource Version"
         , style <| Resource.Styles.pinButton pinState
         ]
            ++ eventHandlers
        )
        (case pinState of
            PinnedStatically { showTooltip } ->
                if showTooltip then
                    [ Html.div
                        [ style
                            [ ( "position", "absolute" )
                            , ( "bottom", "25px" )
                            , ( "background-color", Colors.tooltipBackground )
                            , ( "z-index", "2" )
                            , ( "padding", "5px" )
                            , ( "width", "170px" )
                            ]
                        ]
                        [ Html.text "enable via pipeline config" ]
                    ]

                else
                    []

            InTransition ->
                [ Html.fromUnstyled <|
                    Spinner.spinner
                        "12.5px"
                        [ Html.Attributes.style [ ( "margin", "6.25px" ) ] ]
                ]

            _ ->
                []
        )


viewVersionHeader :
    { a
        | id : Models.VersionId
        , version : Concourse.Version
        , pinnedState : VersionPinState
    }
    -> Html Msg
viewVersionHeader { id, version, pinnedState } =
    Html.div
        [ onClick <| ExpandVersionedResource id
        , style <| Resource.Styles.versionHeader pinnedState
        ]
        [ viewVersion [] version ]


viewVersion : List (UnstyledHtml.Attribute Msg) -> Concourse.Version -> Html Msg
viewVersion attrs version =
    version
        |> Dict.map (always Html.text)
        |> Dict.map (always Html.toUnstyled)
        |> DictView.view attrs
        |> Html.fromUnstyled


viewMetadata : Concourse.Metadata -> Html Msg
viewMetadata metadata =
    Html.dl [ class "build-metadata" ]
        (List.concatMap viewMetadataField metadata)


viewMetadataField : Concourse.MetadataField -> List (Html a)
viewMetadataField field =
    [ Html.dt [] [ Html.text field.name ]
    , Html.dd []
        [ Html.pre [ class "metadata-field" ] [ Html.text field.value ]
        ]
    ]


listToMap : List Concourse.Build -> Dict.Dict String (List Concourse.Build)
listToMap builds =
    let
        insertBuild =
            \build dict ->
                let
                    jobName =
                        case build.job of
                            Nothing ->
                                Debug.crash "Jobless builds shouldn't appear on this page!" ""

                            Just job ->
                                job.jobName

                    oldList =
                        Dict.get jobName dict

                    newList =
                        case oldList of
                            Nothing ->
                                [ build ]

                            Just list ->
                                list ++ [ build ]
                in
                Dict.insert jobName newList dict
    in
    List.foldr insertBuild Dict.empty builds


viewBuilds : Dict.Dict String (List Concourse.Build) -> List (Html Msg)
viewBuilds buildDict =
    List.concatMap (viewBuildsByJob buildDict) <| Dict.keys buildDict


viewLastChecked : Time -> Date -> Html a
viewLastChecked now date =
    let
        ago =
            Duration.between (Date.toTime date) now
    in
    Html.table [ id "last-checked" ]
        [ Html.tr
            []
            [ Html.td [] [ Html.text "checked" ]
            , Html.td [ title (Date.Format.format "%b %d %Y %I:%M:%S %p" date) ]
                [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
            ]
        ]


viewBuildsByJob : Dict.Dict String (List Concourse.Build) -> String -> List (Html Msg)
viewBuildsByJob buildDict jobName =
    let
        oneBuildToLi =
            \build ->
                case build.job of
                    Nothing ->
                        Html.li [ class <| Concourse.BuildStatus.show build.status ]
                            [ Html.text <| "#" ++ build.name ]

                    Just job ->
                        let
                            link =
                                Routes.Build
                                    { id =
                                        { teamName = job.teamName
                                        , pipelineName = job.pipelineName
                                        , jobName = job.jobName
                                        , buildName = build.name
                                        }
                                    , highlight = Routes.HighlightNothing
                                    }
                        in
                        Html.li [ class <| Concourse.BuildStatus.show build.status ]
                            [ Html.a
                                [ Html.Styled.Attributes.fromUnstyled <| StrictEvents.onLeftClick <| NavTo link
                                , href (Routes.toString link)
                                ]
                                [ Html.text <| "#" ++ build.name ]
                            ]
    in
    [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
    , Html.ul [ class "builds-list" ]
        (case Dict.get jobName buildDict of
            Nothing ->
                []

            -- never happens
            Just buildList ->
                List.map oneBuildToLi buildList
        )
    ]


fetchDataForExpandedVersions : Model -> List Effect
fetchDataForExpandedVersions model =
    model.versions.content
        |> List.filter .expanded
        |> List.concatMap (\v -> [ FetchInputTo v.id, FetchOutputOf v.id ])
